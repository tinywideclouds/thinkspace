package integration_test

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/db"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// --- Mocks ---

type mockSandbox struct{}

func (m *mockSandbox) WriteFile(ctx context.Context, path string, data []byte) error { return nil }
func (m *mockSandbox) ReadFile(ctx context.Context, path string) ([]byte, error) {
	return []byte(`{"trace": "ok"}`), nil
}
func (m *mockSandbox) ExecuteCommand(ctx context.Context, command string, args ...string) (string, error) {
	return "", nil
}
func (m *mockSandbox) ApplyDraft(ctx context.Context, message string) error { return nil }
func (m *mockSandbox) DeliverForReview(ctx context.Context) error           { return nil }
func (m *mockSandbox) TearDown(ctx context.Context) error                   { return nil }

type mockChatEngine struct{}

func (engine *mockChatEngine) InitChat(ctx context.Context, chatID string) error { return nil }
func (engine *mockChatEngine) Snapshot(ctx context.Context, chatID, msg string) (string, error) {
	return "mock-sha", nil
}
func (engine *mockChatEngine) SpawnCandidateSandbox(ctx context.Context, chatID, candidateID string) (workspace.CandidateSandbox, error) {
	return &mockSandbox{}, nil
}
func (engine *mockChatEngine) PreviewCandidate(ctx context.Context, chatID, candidateID string) error {
	return nil
}
func (engine *mockChatEngine) Accept(ctx context.Context, chatID, candidateID, reason string) error {
	return nil
}
func (engine *mockChatEngine) Reject(ctx context.Context, chatID, candidateID, reason string) error {
	return nil
}
func (engine *mockChatEngine) ReadCandidateDiff(ctx context.Context, chatID, candidateID string) (string, error) {
	return "", nil
}

type mockDeterministicLLM struct {
	Responses []llm.StreamChunk
	callIndex int
}

func (m *mockDeterministicLLM) GenerateContentStream(
	ctx context.Context,
	model string,
	bundle cognition.ThinkBundle,
	registry *tools.Registry,
	config spaces.ThinkStateConfig,
) iter.Seq2[llm.StreamChunk, error] {
	return func(yield func(llm.StreamChunk, error) bool) {
		if m.callIndex < len(m.Responses) {
			yield(m.Responses[m.callIndex], nil)
			m.callIndex++
		}
	}
}

func (m *mockDeterministicLLM) GenerateWorkerStream(ctx context.Context, model string, bundle cognition.ThinkBundle) iter.Seq2[llm.StreamChunk, error] {
	return m.GenerateContentStream(ctx, model, bundle, nil, spaces.ThinkStateConfig{})
}

type mockThinkState struct{}

func (m *mockThinkState) Config() spaces.ThinkStateConfig {
	cfg := spaces.ThinkStateConfig{
		Timeouts: spaces.TimeoutsConfig{TurnSeconds: 10},
	}
	cfg.ApplyDefaults()
	return cfg
}
func (m *mockThinkState) Verifier() spaces.Verifier { return m }
func (m *mockThinkState) Verify(ctx context.Context, sandbox workspace.CandidateSandbox) error {
	return nil
}
func (m *mockThinkState) Mapbook() spaces.Mapbook { return m }
func (m *mockThinkState) GenerateLayers(ctx context.Context, workspaceRoot string) (map[spaces.MapbookLayer]string, error) {
	return map[spaces.MapbookLayer]string{}, nil
}
func (m *mockThinkState) Patcher() workspace.Patcher                             { return nil }
func (m *mockThinkState) Initialize(ctx context.Context, directory string) error { return nil }
func (m *mockThinkState) EnrichBundle(ctx context.Context, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus, workspaceRoot string) error {
	return nil
}

type mockUI struct{}

func (ui *mockUI) OnTextChunk(text string) {}
func (ui *mockUI) ReviewContext(turnContext session.TurnContext) session.ContextApprovalDecision {
	return session.ContextApprovalDecision{Approved: true}
}
func (ui *mockUI) ChooseNextStep() flows.FlowStrategy                               { return flows.StrategyManual }
func (ui *mockUI) ReviewCandidate(branch string) bool                               { return true }
func (ui *mockUI) ConfirmTransition(targetThinkState string, reasoning string) bool { return true }
func (ui *mockUI) GetAgentTokenChannel() chan<- workspace.AgentToken                { return nil }

type mockFlow struct{ step int }

func (f *mockFlow) Name() string  { return "MockFlow" }
func (f *mockFlow) MaxSteps() int { return 2 }
func (f *mockFlow) Next(state flows.FlowState) flows.FlowDirective {
	if f.step == 0 {
		f.step++
		return flows.FlowDirective{
			Action: flows.ActionSpawnAgents,
			Tasks:  []flows.SubAgentTask{{AgentID: "1", Instruction: "mock"}},
		}
	}
	return flows.FlowDirective{Action: flows.ActionFinish}
}

type mockEmitter struct{}

func (m *mockEmitter) Emit(channel string, event telemetry.Event) {}

// --- Tests ---

func TestEndToEndStateLifecycle(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()
	spaceID := "integration-space"
	chatID := "lifecycle-chat"

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("Failed to init BadgerDB: %v", err)
	}
	defer graphDB.Close()

	ledger := thinkgraph.NewFileLedger(1000, workspaceRoot)
	graphStore := thinkgraph.NewGraphStore(graphDB, nil)
	dispatcher := thinkgraph.NewDispatcher(graphStore, ledger)

	svc := workspace.NewService(logger, &mockChatEngine{}, workspaceRoot, dispatcher, nil)
	chatSession, err := svc.InitializeChat(ctx, spaceID, chatID)
	if err != nil {
		t.Fatalf("Failed to initialize chat: %v", err)
	}

	var targetEvents []thinkgraph.Event

	t.Run("Act I: The Conversation", func(t *testing.T) {
		if err := svc.LogUserPrompt(ctx, chatSession, "How do I build a server?"); err != nil {
			t.Fatalf("LogUserPrompt failed: %v", err)
		}
		time.Sleep(1 * time.Millisecond)
		if err := svc.LogModelResponse(ctx, chatSession, "Use net/http.", nil); err != nil {
			t.Fatalf("LogModelResponse failed: %v", err)
		}

		timeline, err := graphDB.GetBranchTimeline(ctx, chatSession, chatSession.ActiveBranch)
		if err != nil {
			t.Fatalf("GetBranchTimeline failed: %v", err)
		}
		if len(timeline) != 2 {
			t.Fatalf("Expected 2 events in graph, got %d", len(timeline))
		}

		for _, n := range timeline {
			targetEvents = append(targetEvents, n.Event)
		}
	})

	t.Run("Act II: The Compression", func(t *testing.T) {
		mockJSON := fmt.Sprintf(`{
			"thought": "Summarizing the server setup",
			"digests": [
				{
					"topic_name": "Server Setup",
					"tags": ["golang", "http"],
					"summary": "User asked how to build a server. Model suggested net/http."
				}
			]
		}`)

		mockLLM := &mockDeterministicLLM{Responses: []llm.StreamChunk{{Text: mockJSON}}}
		digestManager := session.NewDigestManager(logger, mockLLM, session.DigestConfig{})

		count, err := digestManager.CreateBackgroundDigest(ctx, svc, chatSession, targetEvents, nil, true, "mock-model", "persona")
		if err != nil || count != 1 {
			t.Fatalf("Failed to create digest: %v", err)
		}

		digestsDir := filepath.Join(workspaceRoot, spaceID, "chats", chatID, "ledger", "digests")
		entries, err := os.ReadDir(digestsDir)
		if err != nil {
			t.Fatalf("Failed to read digests directory: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("Expected 1 markdown digest on disk, got %d", len(entries))
		}

		graphDigests, err := graphDB.GetDigests(ctx, chatSession)
		if err != nil {
			t.Fatalf("GetDigests failed: %v", err)
		}
		if len(graphDigests) != 1 {
			t.Fatalf("Expected 1 digest in graph, got %d", len(graphDigests))
		}
	})

	t.Run("Act III: Amnesia & Rehydration", func(t *testing.T) {
		if err := graphDB.ClearChatGraph(ctx, chatSession); err != nil {
			t.Fatalf("ClearChatGraph failed: %v", err)
		}
		timeline, err := graphDB.GetBranchTimeline(ctx, chatSession, chatSession.ActiveBranch)
		if err != nil {
			t.Fatalf("GetBranchTimeline after clear failed: %v", err)
		}
		if len(timeline) != 0 {
			t.Fatalf("Failed to wipe graph: %d events remain", len(timeline))
		}

		rehydrator := thinkgraph.NewRehydrator(graphStore, workspaceRoot)
		if err := rehydrator.Rebuild(ctx, chatSession); err != nil {
			t.Fatalf("Rehydration failed: %v", err)
		}

		restoredTimeline, err := graphDB.GetBranchTimeline(ctx, chatSession, chatSession.ActiveBranch)
		if err != nil {
			t.Fatalf("GetBranchTimeline after rehydration failed: %v", err)
		}
		if len(restoredTimeline) != 2 {
			t.Fatalf("Expected 2 events restored, got %d", len(restoredTimeline))
		}
		if restoredTimeline[0].ID != targetEvents[0].ID {
			t.Errorf("Timeline ordering corrupted during rehydration")
		}

		restoredDigests, err := graphDB.GetDigests(ctx, chatSession)
		if err != nil {
			t.Fatalf("GetDigests after rehydration failed: %v", err)
		}
		if len(restoredDigests) != 1 {
			t.Fatalf("Expected 1 digest restored, got %d", len(restoredDigests))
		}

		restoredDigest := restoredDigests[0]
		if restoredDigest.Summary != "User asked how to build a server. Model suggested net/http." {
			t.Errorf("Digest summary corrupted")
		}

		if len(restoredDigest.SummarizedEvents) != 2 {
			t.Fatalf("Expected 2 linked events, got %d", len(restoredDigest.SummarizedEvents))
		}
		if restoredDigest.SummarizedEvents[0] != targetEvents[0].ID {
			t.Errorf("Linkage lost: Expected %s, got %s", targetEvents[0].ID, restoredDigest.SummarizedEvents[0])
		}
	})

	t.Run("Act IV: Tool Delegation & Flow Resolution", func(t *testing.T) {
		toolLLM := &mockDeterministicLLM{
			Responses: []llm.StreamChunk{
				{
					ToolCalls: []llm.ToolCall{{
						Name: "propose_change",
						Args: map[string]any{
							"instruction":    "Write tests",
							"context_digest": "Testing context",
							"target_files":   []any{},
							"agent_count":    1,
						},
					}},
				},
			},
		}

		executor := func(ctx context.Context, bundle cognition.ThinkBundle, sandbox workspace.CandidateSandbox, agentID int, tokenChan chan<- workspace.AgentToken) error {
			return nil
		}
		emitter := &mockEmitter{}
		flowEngine := session.NewFlowEngine(logger, svc, executor, dispatcher, emitter)

		registry := tools.NewRegistry()
		thinkState := &mockThinkState{}
		ui := &mockUI{}

		coordinator := session.NewTurnCoordinator(logger, svc, toolLLM, emitter, flowEngine, &mockFlow{}, nil, spaceID, "worker prompt", "worker model", registry, thinkState, ui)
		registry.Register(tools.NewProposeChangeTool(coordinator))

		err := coordinator.RunAgentLoop(ctx, chatSession, nil, thinkState, cognition.ThinkBundle{})
		if err != nil {
			t.Fatalf("RunAgentLoop failed: %v", err)
		}

		timeline, err := graphDB.GetBranchTimeline(ctx, chatSession, chatSession.ActiveBranch)
		if err != nil {
			t.Fatalf("GetBranchTimeline failed: %v", err)
		}

		foundLifecycle := false
		for _, n := range timeline {
			if n.Event.Type == flows.EventFlowLifecycle {
				foundLifecycle = true
				break
			}
		}

		if !foundLifecycle {
			t.Errorf("Expected EventFlowLifecycle to be appended to graph timeline after propose_change")
		}
	})
}
