package session_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/dgraph-io/badger/v4"
	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/db"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type fsMockChatEngine struct {
	mu        sync.Mutex
	rejectErr error
	accepted  []string
	rejected  []struct {
		id     string
		reason string
	}
}

func (m *fsMockChatEngine) InitChat(ctx context.Context, chatID string) error { return nil }
func (m *fsMockChatEngine) Snapshot(ctx context.Context, chatID, message string) (string, error) {
	return "", nil
}
func (m *fsMockChatEngine) SpawnCandidateSandbox(ctx context.Context, chatID, candidateID string) (workspace.CandidateSandbox, error) {
	return nil, nil
}
func (m *fsMockChatEngine) PreviewCandidate(ctx context.Context, chatID, candidateID string) error {
	return nil
}
func (m *fsMockChatEngine) ReadCandidateDiff(ctx context.Context, chatID, candidateID string) (string, error) {
	return "", nil
}
func (m *fsMockChatEngine) Accept(ctx context.Context, chatID, candidateID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accepted = append(m.accepted, candidateID)
	return nil
}
func (m *fsMockChatEngine) Reject(ctx context.Context, chatID, candidateID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rejectErr != nil {
		return m.rejectErr
	}
	m.rejected = append(m.rejected, struct {
		id     string
		reason string
	}{id: candidateID, reason: reason})
	return nil
}

func TestFlowScope_ConcurrentRegisterResolveAndIdempotentFinalize(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("failed to init badger: %v", err)
	}
	defer graphDB.Close()

	engine := &fsMockChatEngine{}
	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatVersioner { return engine }, graphDB, nil)
	service := svcManager.GetService("golang")

	chatAddr := &thinkgraph.ChatAddress{
		ChatID:    "chat-scope",
		ProjectID: "golang",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}
	if err := service.InitializeChat(ctx, chatAddr); err != nil {
		t.Fatalf("InitializeChat failed: %v", err)
	}

	flowID := uuid.NewV7()
	flowAddr := &thinkgraph.ChatAddress{
		ChatID:    "chat-scope",
		ProjectID: "golang",
		SubGraph: thinkgraph.SubGraph{
			Name:    "flow-" + flowID.String(),
			GraphID: thinkgraph.GraphID{Prefix: thinkgraph.PrefixFlow, UUID: flowID},
		},
	}

	flowEv := thinkgraph.NewEvent(flows.EventFlowLifecycle, "Flow started")
	flowEv.ID = flowID
	if err := service.Dispatcher().AppendEvent(ctx, chatAddr, flowEv); err != nil {
		t.Fatalf("failed to append flow lifecycle node: %v", err)
	}

	broadcaster := session.NewGraphBroadcasterFromService(logger, service)
	scope := session.NewFlowScope(logger, service, broadcaster, chatAddr, flowAddr, flowID)

	var wg sync.WaitGroup
	ids := []string{"cand-1", "candidate/cand-2", "cand-synthesis"}
	for i, id := range ids {
		wg.Add(1)
		go func(idx int, candID string) {
			defer wg.Done()
			scope.RegisterCandidate(flows.Candidate{
				ID:      candID,
				AgentID: fmt.Sprintf("agent-%d", idx+1),
				Passed:  true,
			})
		}(i, id)
	}
	wg.Wait()

	if len(scope.Candidates()) != 3 {
		t.Fatalf("expected 3 registered candidates, got %d", len(scope.Candidates()))
	}

	if err := scope.ResolveCandidate(ctx, "cand-synthesis", true, "Merged winner"); err != nil {
		t.Fatalf("ResolveCandidate accept failed: %v", err)
	}
	if err := scope.ResolveCandidate(ctx, "cand-2", false, "Explicitly rejected"); err != nil {
		t.Fatalf("ResolveCandidate reject failed: %v", err)
	}

	if err := scope.Finalize(ctx, "Superseded by synthesis"); err != nil {
		t.Fatalf("Finalize failed: %v", err)
	}
	if err := scope.Finalize(ctx, "Duplicate finalize"); err != nil {
		t.Fatalf("Duplicate Finalize failed: %v", err)
	}

	if len(engine.accepted) != 1 || engine.accepted[0] != "cand-synthesis" {
		t.Errorf("expected only cand-synthesis to be accepted, got %v", engine.accepted)
	}
	if len(engine.rejected) != 2 {
		t.Fatalf("expected exactly 2 rejected candidates (cand-2 explicit + cand-1 finalized), got %d: %+v", len(engine.rejected), engine.rejected)
	}

	rejectedReasons := map[string]string{}
	for _, r := range engine.rejected {
		rejectedReasons[r.id] = r.reason
	}
	if rejectedReasons["cand-2"] != "Explicitly rejected" {
		t.Errorf("expected cand-2 reason 'Explicitly rejected', got %q", rejectedReasons["cand-2"])
	}
	if rejectedReasons["cand-1"] != "Superseded by synthesis" {
		t.Errorf("expected cand-1 reason 'Superseded by synthesis', got %q", rejectedReasons["cand-1"])
	}

	if err := scope.Complete(ctx, "All done", []flows.AgentRecord{{AgentID: "agent-1", Passed: true}}, nil); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	nodes, err := service.Graph().GetNodes(ctx, chatAddr, []string{flowID.String()})
	if err != nil || len(nodes) != 1 {
		t.Fatalf("expected flow node, got %d (err=%v)", len(nodes), err)
	}
	if nodes[0].Content != "All done" {
		t.Errorf("expected flow node content 'All done', got %q", nodes[0].Content)
	}
	var st flows.FlowExecutionState
	if err := json.Unmarshal(nodes[0].MaterializedState, &st); err != nil {
		t.Fatalf("failed to unmarshal flow execution state: %v", err)
	}
	if st.Status != flows.FlowStatusCompleted {
		t.Errorf("expected FlowStatusCompleted, got %v", st.Status)
	}
}

func TestFlowScope_PropagatesErrors(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("failed to init badger: %v", err)
	}
	defer graphDB.Close()

	engine := &fsMockChatEngine{rejectErr: errors.New("git lock error")}
	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatVersioner { return engine }, graphDB, nil)
	service := svcManager.GetService("golang")

	chatAddr := &thinkgraph.ChatAddress{
		ChatID:    "chat-err",
		ProjectID: "golang",
		SubGraph:  thinkgraph.SubGraph{Name: "main", GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat)},
	}
	if err := service.InitializeChat(ctx, chatAddr); err != nil {
		t.Fatalf("InitializeChat failed: %v", err)
	}

	missingFlowID := uuid.NewV7()
	broadcaster := session.NewGraphBroadcasterFromService(logger, service)
	scope := session.NewFlowScope(logger, service, broadcaster, chatAddr, nil, missingFlowID)

	if err := scope.Complete(ctx, "Summary", nil, nil); err == nil {
		t.Errorf("expected Complete to return an error when target flow node is missing from graph")
	}

	scope.RegisterCandidate(flows.Candidate{ID: "cand-fail"})
	if err := scope.Finalize(ctx, "Cleanup"); err == nil {
		t.Errorf("expected Finalize to return joined error when Reject fails")
	}
}
