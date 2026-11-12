package integration_test

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v4"
	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/db"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

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

type mockThinkState struct{}

func (m *mockThinkState) Config() spaces.ThinkStateConfig {
	cfg := spaces.ThinkStateConfig{Timeouts: spaces.TimeoutsConfig{TurnSeconds: 10}}
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

func (ui *mockUI) OnTextChunk(chunk workspace.NodeChunk) {}
func (ui *mockUI) StreamGraphEvent(node thinkgraph.Node) {}
func (ui *mockUI) ReviewContext(turnContext session.TurnContext) session.ContextApprovalDecision {
	return session.ContextApprovalDecision{Approved: true}
}
func (ui *mockUI) ChooseNextStep() flows.FlowStrategy                  { return flows.StrategyManual }
func (ui *mockUI) ReviewCandidate(branch string) bool                  { return true }
func (ui *mockUI) ConfirmTransition(req signals.SignalTransition) bool { return true }
func (ui *mockUI) GetChunkChannel() chan<- workspace.NodeChunk         { return nil }

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

	registry := config.NewRegistry()
	mockSpace := &mockThinkState{}
	registry.RegisterThinkState("golang", mockSpace)
	registry.RegisterConfig("golang", mockSpace.Config())

	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(s string) workspace.ChatVersioner { return &mockChatEngine{} }, graphDB, nil)
	_ = os.MkdirAll(filepath.Join(workspaceRoot, spaceID), 0755)
	_ = svcManager.UpdateSpaceState(spaceID, func(s *workspace.SpaceState) { s.ThinkState = "golang" })

	graphID := thinkgraph.NewGraphID(thinkgraph.PrefixChat)
	metaJSON := fmt.Sprintf(`{"id":"%s", "name":"Lifecycle Chat", "active_branch": "main"}`, chatID)

	chatDir := filepath.Join(workspaceRoot, spaceID, "chats", chatID)
	_ = os.MkdirAll(chatDir, 0755)
	_ = os.WriteFile(filepath.Join(chatDir, "meta.json"), []byte(metaJSON), 0644)

	svc := svcManager.GetService(spaceID)
	chatAddress := &thinkgraph.ChatAddress{
		ChatID: chatID, ProjectID: spaceID, SubGraph: thinkgraph.SubGraph{Name: "main", GraphID: graphID},
	}
	if err := svc.InitializeChat(ctx, chatAddress); err != nil {
		t.Fatalf("Failed to initialize chat: %v", err)
	}

	var targetEvents []thinkgraph.Event
	var integrationChatSession *session.ChatSession

	t.Run("Act I: The Conversation", func(t *testing.T) {
		if err := svc.AppendUserPrompt(ctx, chatAddress, "How do I build a server?"); err != nil {
			t.Fatalf("LogUserPrompt failed: %v", err)
		}
		time.Sleep(1 * time.Millisecond)
		if err := svc.AppendModelResponse(ctx, chatAddress, thinkgraph.NewGraphID(thinkgraph.PrefixChat).UUID, "Use net/http.", nil); err != nil {
			t.Fatalf("LogModelResponse failed: %v", err)
		}

		timeline, err := graphDB.GetSubGraphTimeline(ctx, chatAddress, chatAddress.GraphID)
		if err != nil {
			t.Fatalf("GetSubGraphTimeline failed: %v", err)
		}
		if len(timeline) != 2 {
			t.Fatalf("Expected 2 events in graph, got %d", len(timeline))
		}

		for _, n := range timeline {
			targetEvents = append(targetEvents, n.Event)
		}
	})

	t.Run("Act II: The Compression", func(t *testing.T) {
		mockJSON := `{
			"thought": "Summarizing the server setup",
			"digests": [
				{
					"topic_name": "Server Setup",
					"tags": ["golang", "http"],
					"summary": "User asked how to build a server. Model suggested net/http."
				}
			]
		}`

		mockLLM := &mockDeterministicLLM{Responses: []llm.StreamChunk{{Text: mockJSON}}}
		digestManager := session.NewDigestManager(logger, mockLLM, session.DigestConfig{})

		count, err := digestManager.CreateBackgroundDigest(ctx, svc, chatAddress, targetEvents, nil, true, "mock-model", "persona")
		if err != nil || count != 1 {
			t.Fatalf("Failed to create digest: %v", err)
		}

		graphDigests, err := graphDB.GetDigests(ctx, chatAddress)
		if err != nil {
			t.Fatalf("GetDigests failed: %v", err)
		}
		if len(graphDigests) != 1 {
			t.Fatalf("Expected 1 digest in graph, got %d", len(graphDigests))
		}
	})

	t.Run("Act III: Amnesia & Rehydration", func(t *testing.T) {
		if err := graphDB.ClearChatGraph(ctx, chatAddress); err != nil {
			t.Fatalf("ClearChatGraph failed: %v", err)
		}

		rehydrator := thinkgraph.NewRehydrator(svc.Dispatcher().GraphStore(), workspaceRoot)
		if err := rehydrator.Rebuild(ctx, chatAddress); err != nil {
			t.Fatalf("Rehydration failed: %v", err)
		}

		restoredTimeline, err := graphDB.GetSubGraphTimeline(ctx, chatAddress, chatAddress.GraphID)
		if err != nil {
			t.Fatalf("GetSubGraphTimeline after rehydration failed: %v", err)
		}
		if len(restoredTimeline) != 2 {
			t.Fatalf("Expected 2 events restored, got %d", len(restoredTimeline))
		}

		restoredDigests, err := graphDB.GetDigests(ctx, chatAddress)
		if err != nil {
			t.Fatalf("GetDigests after rehydration failed: %v", err)
		}
		if len(restoredDigests) != 1 {
			t.Fatalf("Expected 1 digest restored, got %d", len(restoredDigests))
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

		emitter := &mockEmitter{}
		ui := &mockUI{}
		toolRegistry := tools.NewRegistry()
		toolRegistry.Register(tools.NewProposeChangeTool())

		turnExecutor := session.NewTurnExecutor(logger, toolLLM, svc, emitter)

		responseID := uuid.NewV7()
		// Fixed: Execute matches (ctx, address, responseID, thinkState, bundle, registry, ui)
		signal, err := turnExecutor.Execute(ctx, chatAddress, responseID, mockSpace, cognition.ThinkBundle{}, toolRegistry, ui)
		if err != nil {
			t.Fatalf("TurnExecutor failed: %v", err)
		}

		dispatchSig, ok := signal.(*signals.SignalDispatchFlow)
		if !ok {
			t.Fatalf("Expected TurnExecutor to yield a SignalDispatchFlow, got %T", signal)
		}

		digestManager := session.NewDigestManager(logger, toolLLM, session.DigestConfig{})
		integrationChatSession, err = session.NewChatSession(ctx, logger, registry, svcManager, toolLLM, &mockFlow{}, digestManager, spaceID, chatID)
		if err != nil {
			t.Fatalf("Failed to spin up ChatSession supervisor: %v", err)
		}

		workerEngine := session.NewWorkerEngine(logger, integrationChatSession, registry, svc, turnExecutor, svc.Dispatcher(), &mockFlow{})

		_, err = workerEngine.Execute(ctx, chatAddress, responseID, uuid.NewV7(), dispatchSig, ui, emitter)
		if err != nil {
			t.Fatalf("WorkerEngine failed: %v", err)
		}

		timeline, err := graphDB.GetSubGraphTimeline(ctx, chatAddress, chatAddress.GraphID)
		if err != nil {
			t.Fatalf("GetSubGraphTimeline failed: %v", err)
		}

		foundLifecycle := false
		for _, n := range timeline {
			if n.Event.Type == flows.EventFlowLifecycle {
				foundLifecycle = true
				break
			}
		}

		if !foundLifecycle {
			t.Errorf("Expected EventFlowLifecycle to be appended to graph timeline after WorkerEngine completed execution")
		}
	})

	t.Run("Act V: System Transition & Turn Reboot", func(t *testing.T) {
		registry.RegisterThinkState("concept", mockSpace)
		registry.RegisterConfig("concept", mockSpace.Config())

		systemEngine := session.NewSystemEngine(logger, integrationChatSession, registry, svc)

		transitionSig := &signals.SignalTransition{
			TargetThinkState: "concept",
			Reasoning:        "Moving to ideation phase",
		}

		turnID := uuid.NewV7()
		catalystID := uuid.NewV7()
		nextSignal, err := systemEngine.Execute(ctx, chatAddress, turnID, catalystID, transitionSig, &mockUI{}, &mockEmitter{})
		if err != nil {
			t.Fatalf("SystemEngine failed: %v", err)
		}

		if _, ok := nextSignal.(*signals.SignalExecuteChat); !ok {
			t.Fatalf("Expected SystemEngine to yield SignalExecuteChat to reboot the turn, got %T", nextSignal)
		}

		meta, err := svcManager.GetChatMeta(spaceID, chatID)
		if err != nil {
			t.Fatalf("Failed to fetch chat meta: %v", err)
		}
		if meta.ActiveThinkState != "concept" {
			t.Errorf("Expected active think state to be explicitly mutated to 'concept', got '%s'", meta.ActiveThinkState)
		}

		sysAddr := integrationChatSession.GetSystemAddress()
		sysTimeline, err := svc.Graph().GetSubGraphTimeline(ctx, sysAddr, sysAddr.GraphID)
		if err != nil {
			t.Fatalf("Failed to fetch system timeline: %v", err)
		}
		foundSystemEvent := false
		for _, n := range sysTimeline {
			if n.Type == thinkgraph.EventSystem && strings.Contains(n.Content, "Workspace transitioned to concept") {
				foundSystemEvent = true
				break
			}
		}
		if !foundSystemEvent {
			t.Errorf("Expected transition EventSystem node to be appended to 'system' subgraph timeline")
		}
	})
}
