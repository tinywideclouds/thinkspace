package session_test

import (
	"context"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

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

// --- Shared Mocks ---

type tmTestModelClient struct {
	responses []llm.StreamChunk
	callCount int
}

func (m *tmTestModelClient) GenerateContentStream(ctx context.Context, model string, bundle cognition.ThinkBundle, tools []llm.ToolDef) iter.Seq2[llm.StreamChunk, error] {
	return func(yield func(llm.StreamChunk, error) bool) {
		if m.callCount < len(m.responses) {
			resp := m.responses[m.callCount]
			m.callCount++
			yield(resp, nil)
		}
	}
}

type tmTestChatEngine struct{}

func (m *tmTestChatEngine) InitChat(ctx context.Context, chatID string) error { return nil }
func (m *tmTestChatEngine) Snapshot(ctx context.Context, chatID string, message string) (string, error) {
	return "mock-sha", nil
}
func (m *tmTestChatEngine) SpawnCandidateSandbox(ctx context.Context, chatID, candidateID string) (workspace.CandidateSandbox, error) {
	return nil, nil
}
func (m *tmTestChatEngine) PreviewCandidate(ctx context.Context, chatID, candidateID string) error {
	return nil
}
func (m *tmTestChatEngine) Accept(ctx context.Context, chatID, candidateID, reason string) error {
	return nil
}
func (m *tmTestChatEngine) Reject(ctx context.Context, chatID, candidateID, reason string) error {
	return nil
}
func (m *tmTestChatEngine) ReadCandidateDiff(ctx context.Context, chatID, candidateID string) (string, error) {
	return "", nil
}

type tmTestThinkSpace struct{}

func (m *tmTestThinkSpace) Config() spaces.ThinkStateConfig {
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
func (m *tmTestThinkSpace) Tools() []llm.ToolDef                                   { return nil }
func (m *tmTestThinkSpace) Verifier() spaces.Verifier                              { return nil }
func (m *tmTestThinkSpace) Mapbook() spaces.Mapbook                                { return nil }
func (m *tmTestThinkSpace) Patcher() workspace.Patcher                             { return nil }
func (m *tmTestThinkSpace) Initialize(ctx context.Context, directory string) error { return nil }
func (m *tmTestThinkSpace) EnrichBundle(ctx context.Context, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus, workspaceRoot string) error {
	return nil
}

type tmTestUI struct {
	reviewApproved         bool
	retainedHistoryIndices []int
	retainedFilePaths      []string
	transitionApproved     bool
}

func (u *tmTestUI) OnTextChunk(text string) {}
func (u *tmTestUI) ReviewContext(ctxData session.TurnContext) session.ContextApprovalDecision {
	return session.ContextApprovalDecision{
		Approved:               u.reviewApproved,
		RetainedHistoryIndices: u.retainedHistoryIndices,
		RetainedFilePaths:      u.retainedFilePaths,
	}
}
func (u *tmTestUI) ChooseNextStep() flows.FlowStrategy { return flows.StrategyManual }
func (u *tmTestUI) ReviewCandidate(branch string) bool { return true }
func (u *tmTestUI) ConfirmTransition(targetDomain string, reasoning string) bool {
	return u.transitionApproved
}

type tmTestEmitter struct{}

func (e *tmTestEmitter) Emit(channel string, event telemetry.Event) {}

type tmTestFlow struct{}

func (f *tmTestFlow) Name() string  { return "TestFlow" }
func (f *tmTestFlow) MaxSteps() int { return 10 }
func (f *tmTestFlow) Next(state flows.FlowState) flows.FlowDirective {
	return flows.FlowDirective{Action: flows.ActionFinish}
}

// --- Setup ---

func setupTurnManagerTest(t *testing.T) (*session.TurnManager, *workspace.ServiceManager, *tmTestModelClient, string) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()
	spaceID := "test-space"

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("failed to init badger: %v", err)
	}
	t.Cleanup(func() { graphDB.Close() })

	engineFactory := func(root string) workspace.ChatEngine {
		return &tmTestChatEngine{}
	}

	svcManager := workspace.NewServiceManager(logger, workspaceRoot, engineFactory, graphDB)
	_ = os.MkdirAll(filepath.Join(workspaceRoot, spaceID), 0755)
	_ = svcManager.UpdateSpaceState(spaceID, func(s *workspace.SpaceState) {
		s.Domain = "golang"
	})

	registry := config.NewRegistry()
	mockSpace := &tmTestThinkSpace{}

	registry.RegisterDomain("golang", mockSpace)
	registry.RegisterConfig("golang", mockSpace.Config())

	registry.RegisterDomain("concept", mockSpace)
	registry.RegisterConfig("concept", mockSpace.Config())

	client := &tmTestModelClient{}
	digestManager := session.NewDigestManager(logger, client, session.DigestConfig{})

	tm := session.NewTurnManager(logger, registry, svcManager, client, &tmTestFlow{}, digestManager)
	return tm, svcManager, client, spaceID
}

// --- Tests ---

func TestTurnManager_RunTurn_Sequencing(t *testing.T) {
	ctx := context.Background()
	tm, svcManager, client, spaceID := setupTurnManagerTest(t)
	chatID := "chat-seq"

	// Mock LLM to return standard text (no tools) to cleanly end the turn
	client.responses = []llm.StreamChunk{
		{Text: "Test standard response"},
	}

	ui := &tmTestUI{reviewApproved: true}
	emitter := &tmTestEmitter{}

	err := tm.RunTurn(ctx, spaceID, chatID, "Hello agent", ui, emitter)
	if err != nil {
		t.Fatalf("RunTurn failed: %v", err)
	}

	// Verify Graph DB logged the interaction
	chat := &thinkgraph.Chat{ID: chatID, SpaceID: spaceID}
	timeline, err := svcManager.GraphStore().GetTimeline(ctx, chat)
	if err != nil {
		t.Fatalf("Failed to fetch timeline: %v", err)
	}

	if len(timeline) == 0 {
		t.Errorf("Expected timeline to contain events, got 0")
	}
}

func TestTurnManager_RunTurn_Transition(t *testing.T) {
	ctx := context.Background()
	tm, svcManager, client, spaceID := setupTurnManagerTest(t)
	chatID := "chat-trans"

	// 1st LLM call: Request transition. 2nd LLM call: Normal response in new domain.
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

	ui := &tmTestUI{reviewApproved: true, transitionApproved: true}
	emitter := &tmTestEmitter{}

	err := tm.RunTurn(ctx, spaceID, chatID, "Let's plan", ui, emitter)
	if err != nil {
		t.Fatalf("RunTurn failed: %v", err)
	}

	// Verify ChatMeta was updated to the new domain
	meta, err := svcManager.GetChatMeta(spaceID, chatID)
	if err != nil {
		t.Fatalf("Failed to fetch chat meta: %v", err)
	}
	if meta.ActiveDomain != "concept" {
		t.Errorf("Expected chat to transition to 'concept', got '%s'", meta.ActiveDomain)
	}
}

func TestTurnManager_RunTurn_Pruning(t *testing.T) {
	ctx := context.Background()
	tm, svcManager, client, spaceID := setupTurnManagerTest(t)
	chatID := "chat-prune"

	// Pre-seed some events so we can prune them
	chat := &thinkgraph.Chat{ID: chatID, SpaceID: spaceID}
	svc := svcManager.GetService(spaceID)
	_, _ = svc.InitializeChat(ctx, spaceID, chatID)
	_ = svc.LogUserMessage(ctx, chat, "Message 1")
	_ = svc.LogUserMessage(ctx, chat, "Message 2")
	_ = svc.LogUserMessage(ctx, chat, "Message 3")

	client.responses = []llm.StreamChunk{
		{Text: "Response after pruning"},
	}

	// Approve but strictly retain ONLY index 1 (Message 2) from history
	ui := &tmTestUI{
		reviewApproved:         true,
		retainedHistoryIndices: []int{1},
	}
	emitter := &tmTestEmitter{}

	err := tm.RunTurn(ctx, spaceID, chatID, "Message 4", ui, emitter)
	if err != nil {
		t.Fatalf("RunTurn failed: %v", err)
	}

	// Turn executes without error when context is pruned
	timeline, _ := svcManager.GraphStore().GetTimeline(ctx, chat)
	if len(timeline) < 2 {
		t.Errorf("Expected graph to retain full truth regardless of UI pruning")
	}
}
