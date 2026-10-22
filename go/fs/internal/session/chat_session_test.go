package session_test

import (
	"context"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"uuid"

	"github.com/dgraph-io/badger/v4"
	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/db"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type csTestModelClient struct {
	responses   []llm.StreamChunk
	callCount   int
	lastRequest cognition.ThinkBundle
}

func (m *csTestModelClient) GenerateContentStream(ctx context.Context, model string, bundle cognition.ThinkBundle, tools []llm.ToolDef) iter.Seq2[llm.StreamChunk, error] {
	m.lastRequest = bundle
	return func(yield func(llm.StreamChunk, error) bool) {
		if m.callCount < len(m.responses) {
			resp := m.responses[m.callCount]
			m.callCount++
			yield(resp, nil)
		}
	}
}

type csTestChatEngine struct{}

func (m *csTestChatEngine) InitChat(ctx context.Context, chatID string) error { return nil }
func (m *csTestChatEngine) Snapshot(ctx context.Context, chatID string, message string) (string, error) {
	return "mock-sha", nil
}
func (m *csTestChatEngine) SpawnCandidateSandbox(ctx context.Context, chatID, candidateID string) (workspace.CandidateSandbox, error) {
	return nil, nil
}
func (m *csTestChatEngine) PreviewCandidate(ctx context.Context, chatID, candidateID string) error {
	return nil
}
func (m *csTestChatEngine) Accept(ctx context.Context, chatID, candidateID, reason string) error {
	return nil
}
func (m *csTestChatEngine) Reject(ctx context.Context, chatID, candidateID, reason string) error {
	return nil
}
func (m *csTestChatEngine) ReadCandidateDiff(ctx context.Context, chatID, candidateID string) (string, error) {
	return "", nil
}

type csTestThinkSpace struct{}

func (m *csTestThinkSpace) Config() spaces.ThinkStateConfig {
	cfg := spaces.ThinkStateConfig{
		Roles: spaces.RolesConfig{
			Manager: spaces.ManagerConfig{Model: "test-manager"},
			Worker:  spaces.WorkerConfig{Model: "test-worker"},
		},
		Timeouts: spaces.TimeoutsConfig{TurnSeconds: 300},
	}
	cfg.ApplyDefaults()
	return cfg
}
func (m *csTestThinkSpace) Tools() []llm.ToolDef                                   { return nil }
func (m *csTestThinkSpace) Verifier() spaces.Verifier                              { return nil }
func (m *csTestThinkSpace) Mapbook() spaces.Mapbook                                { return nil }
func (m *csTestThinkSpace) Patcher() workspace.Patcher                             { return nil }
func (m *csTestThinkSpace) Initialize(ctx context.Context, directory string) error { return nil }
func (m *csTestThinkSpace) EnrichBundle(ctx context.Context, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus, workspaceRoot string) error {
	return nil
}

type csTestUI struct {
	reviewApproved         bool
	retainedHistoryIndices []int
	retainedFilePaths      []string
	transitionApproved     bool
}

func (u *csTestUI) OnTextChunk(text string) {}
func (u *csTestUI) ReviewContext(ctxData session.TurnContext) session.ContextApprovalDecision {
	return session.ContextApprovalDecision{
		Approved:               u.reviewApproved,
		RetainedHistoryIndices: u.retainedHistoryIndices,
		RetainedFilePaths:      u.retainedFilePaths,
	}
}
func (u *csTestUI) ChooseNextStep() flows.FlowStrategy { return flows.StrategyManual }
func (u *csTestUI) ReviewCandidate(branch string) bool { return true }
func (u *csTestUI) ConfirmTransition(targetDomain string, reasoning string) bool {
	return u.transitionApproved
}

type csTestEmitter struct{}

func (e *csTestEmitter) Emit(channel string, event telemetry.Event) {}

type csTestFlow struct{}

func (f *csTestFlow) Name() string  { return "TestFlow" }
func (f *csTestFlow) MaxSteps() int { return 10 }
func (f *csTestFlow) Next(state flows.FlowState) flows.FlowDirective {
	return flows.FlowDirective{Action: flows.ActionFinish}
}

func scaffoldTestChat(workspaceRoot, spaceID, chatID string) {
	metaPath := filepath.Join(workspaceRoot, spaceID, "chats", chatID)
	_ = os.MkdirAll(metaPath, 0755)
	_ = os.WriteFile(filepath.Join(metaPath, "meta.json"), []byte(`{"id":"`+chatID+`", "name":"`+chatID+`"}`), 0644)
}

func setupSessionTest(t *testing.T, chatID string) (*session.ChatSession, *workspace.ServiceManager, *csTestModelClient, string) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()
	spaceID := "test-space"

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("failed to init badger: %v", err)
	}
	t.Cleanup(func() { graphDB.Close() })

	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatEngine { return &csTestChatEngine{} }, graphDB)
	_ = os.MkdirAll(filepath.Join(workspaceRoot, spaceID), 0755)
	_ = svcManager.UpdateSpaceState(spaceID, func(s *workspace.SpaceState) { s.Domain = "golang" })

	scaffoldTestChat(workspaceRoot, spaceID, chatID)

	registry := config.NewRegistry()
	mockSpace := &csTestThinkSpace{}
	registry.RegisterDomain("golang", mockSpace)
	registry.RegisterConfig("golang", mockSpace.Config())
	registry.RegisterDomain("concept", mockSpace)
	registry.RegisterConfig("concept", mockSpace.Config())

	client := &csTestModelClient{}
	digestManager := session.NewDigestManager(logger, client, session.DigestConfig{})

	chatSession, err := session.OpenChat(context.Background(), logger, registry, svcManager, client, &csTestFlow{}, digestManager, spaceID, chatID)
	if err != nil {
		t.Fatalf("failed to open chat session: %v", err)
	}

	return chatSession, svcManager, client, spaceID
}

func TestChatSession_RunTurn_Sequencing(t *testing.T) {
	ctx := context.Background()
	chatID := "chat-seq"
	chatSession, svcManager, client, spaceID := setupSessionTest(t, chatID)

	client.responses = []llm.StreamChunk{{Text: "Test standard response"}}
	ui := &csTestUI{reviewApproved: true}
	emitter := &csTestEmitter{}

	err := chatSession.RunTurn(ctx, "Hello agent", ui, emitter)
	if err != nil {
		t.Fatalf("RunTurn failed: %v", err)
	}

	chat := &thinkgraph.Chat{ID: chatID, SpaceID: spaceID}
	timeline, err := svcManager.GraphStore().GetTimeline(ctx, chat)
	if err != nil {
		t.Fatalf("Failed to fetch timeline: %v", err)
	}

	if len(timeline) == 0 {
		t.Errorf("Expected timeline to contain events, got 0")
	}
}

func TestChatSession_RunTurn_Transition(t *testing.T) {
	ctx := context.Background()
	chatID := "chat-trans"
	chatSession, svcManager, client, spaceID := setupSessionTest(t, chatID)

	client.responses = []llm.StreamChunk{
		{
			ToolCalls: []llm.ToolCall{
				{
					Name: "transition_workspace",
					Args: map[string]any{"target_domain": "concept", "reasoning": "Need planning"},
				},
			},
		},
		{
			Text: "Now executing in concept domain.",
		},
	}

	ui := &csTestUI{reviewApproved: true, transitionApproved: true}
	emitter := &csTestEmitter{}

	err := chatSession.RunTurn(ctx, "Let's plan", ui, emitter)
	if err != nil {
		t.Fatalf("RunTurn failed: %v", err)
	}

	meta, err := svcManager.GetChatMeta(spaceID, chatID)
	if err != nil {
		t.Fatalf("Failed to fetch chat meta: %v", err)
	}
	if meta.ActiveDomain != "concept" {
		t.Errorf("Expected chat to transition to 'concept', got '%s'", meta.ActiveDomain)
	}
}

func TestChatSession_SpaceContextAssembly(t *testing.T) {
	ctx := context.Background()
	chatID := "chat-context"
	chatSession, svcManager, client, spaceID := setupSessionTest(t, chatID)

	chat := &thinkgraph.Chat{ID: chatID, SpaceID: spaceID}
	_ = svcManager.GraphStore().SaveSpaceContext(ctx, chat, thinkgraph.SpaceContext{
		ID:      uuid.NewV7(),
		Content: "Always use PostgreSQL.",
		Tags:    []string{"database"},
	})

	ev := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Build database")
	ev.Tags = []string{"database"}
	_ = svcManager.GraphStore().AppendEvent(ctx, chat, ev)

	client.responses = []llm.StreamChunk{{Text: "Ok"}}
	ui := &csTestUI{reviewApproved: true}
	emitter := &csTestEmitter{}

	_ = chatSession.RunTurn(ctx, "Start", ui, emitter)

	foundRule := false
	for _, memory := range client.lastRequest.Trajectory.LongTermMemory {
		if strings.Contains(memory, "PostgreSQL") {
			foundRule = true
			break
		}
	}

	if !foundRule {
		t.Errorf("ChatSession failed to assemble targeted SpaceContext rules into LLM context bundle")
	}
}
