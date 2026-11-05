package session_test

import (
	"context"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v4"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
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

type coordMockModelClient struct {
	responses      []llm.StreamChunk
	callIndex      int
	capturedBundle cognition.ThinkBundle
}

func (client *coordMockModelClient) GenerateContentStream(
	ctx context.Context,
	model string,
	bundle cognition.ThinkBundle,
	registry *tools.Registry,
	config spaces.ThinkStateConfig,
) iter.Seq2[llm.StreamChunk, error] {
	client.capturedBundle = bundle
	return func(yield func(llm.StreamChunk, error) bool) {
		if client.callIndex < len(client.responses) {
			resp := client.responses[client.callIndex]
			client.callIndex++
			yield(resp, nil)
		}
	}
}

type coordMockChatEngine struct{}

func (engine *coordMockChatEngine) InitChat(ctx context.Context, chatID string) error { return nil }
func (engine *coordMockChatEngine) Snapshot(ctx context.Context, chatID string, message string) (string, error) {
	return "mock-sha", nil
}
func (engine *coordMockChatEngine) SpawnCandidateSandbox(ctx context.Context, chatID string, candidateID string) (workspace.CandidateSandbox, error) {
	return nil, nil
}
func (engine *coordMockChatEngine) PreviewCandidate(ctx context.Context, chatID string, candidateID string) error {
	return nil
}
func (engine *coordMockChatEngine) Accept(ctx context.Context, chatID string, candidateID string, reason string) error {
	return nil
}
func (engine *coordMockChatEngine) Reject(ctx context.Context, chatID string, candidateID string, reason string) error {
	return nil
}
func (engine *coordMockChatEngine) ReadCandidateDiff(ctx context.Context, chatID, candidateID string) (string, error) {
	return "+ mock diff", nil
}

type coordMockThinkSpace struct{}

func (space *coordMockThinkSpace) Config() spaces.ThinkStateConfig {
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
func (space *coordMockThinkSpace) Verifier() spaces.Verifier                              { return nil }
func (space *coordMockThinkSpace) Mapbook() spaces.Mapbook                                { return nil }
func (space *coordMockThinkSpace) Patcher() workspace.Patcher                             { return nil }
func (space *coordMockThinkSpace) Initialize(ctx context.Context, directory string) error { return nil }
func (space *coordMockThinkSpace) EnrichBundle(ctx context.Context, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus, workspaceRoot string) error {
	return nil
}

type coordMockFlow struct {
	executeCalled    bool
	synthesizeOnExec bool
}

func (flow *coordMockFlow) Name() string  { return "MockFlow" }
func (flow *coordMockFlow) MaxSteps() int { return 10 }
func (flow *coordMockFlow) Next(state flows.FlowState) flows.FlowDirective {
	if !flow.executeCalled {
		flow.executeCalled = true
		if flow.synthesizeOnExec {
			return flows.FlowDirective{
				Action: flows.ActionSynthesize,
				Tasks:  []flows.SubAgentTask{{Instruction: "Refine instruction"}},
			}
		}
	}
	return flows.FlowDirective{Action: flows.ActionFinish}
}

type coordMockUI struct {
	strategyToReturn   flows.FlowStrategy
	reviewToReturn     bool
	reviewedBranch     string
	transitionToReturn bool
	capturedReasoning  string
	retainedFiles      []string
	retainedHistory    []int
}

func (ui *coordMockUI) OnTextChunk(text string) {}
func (ui *coordMockUI) ReviewContext(ctxData session.TurnContext) session.ContextApprovalDecision {
	return session.ContextApprovalDecision{
		Approved:               ui.reviewToReturn,
		RetainedFilePaths:      ui.retainedFiles,
		RetainedHistoryIndices: ui.retainedHistory,
	}
}
func (ui *coordMockUI) ChooseNextStep() flows.FlowStrategy { return ui.strategyToReturn }
func (ui *coordMockUI) ReviewCandidate(branch string) bool {
	ui.reviewedBranch = branch
	return ui.reviewToReturn
}
func (ui *coordMockUI) ConfirmTransition(targetDomain string, reasoning string) bool {
	ui.capturedReasoning = reasoning
	return ui.transitionToReturn
}
func (ui *coordMockUI) GetAgentTokenChannel() chan<- workspace.AgentToken { return nil }

type coordEmittedEvent struct {
	channel string
	event   telemetry.Event
}
type coordMockEmitter struct {
	events []coordEmittedEvent
}

func (emitter *coordMockEmitter) Emit(channel string, event telemetry.Event) {
	emitter.events = append(emitter.events, coordEmittedEvent{channel: channel, event: event})
}

func setupCoordTest(t *testing.T, spaceID, chatID string) (*session.ChatSession, *workspace.Service, *coordMockEmitter) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("failed to init badger: %v", err)
	}
	t.Cleanup(func() { graphDB.Close() })

	emitter := &coordMockEmitter{}
	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatVersioner { return &coordMockChatEngine{} }, graphDB, emitter)
	_ = os.MkdirAll(filepath.Join(workspaceRoot, spaceID), 0755)

	scaffoldTestChat(workspaceRoot, spaceID, chatID)

	registry := config.NewRegistry()
	mockSpace := &coordMockThinkSpace{}
	registry.RegisterThinkState(spaceID, mockSpace)
	registry.RegisterConfig(spaceID, mockSpace.Config())

	client := &coordMockModelClient{}
	digestManager := session.NewDigestManager(logger, client, session.DigestConfig{})

	activeSession, err := session.NewChatSession(context.Background(), logger, registry, svcManager, client, &coordMockFlow{}, digestManager, spaceID, chatID)
	if err != nil {
		t.Fatalf("failed to open chat session: %v", err)
	}

	workspaceService := svcManager.GetService(spaceID)

	return activeSession, workspaceService, emitter
}

func TestTurnCoordinator_RunAgentLoop_RoutesToEngine(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	activeSession, workspaceService, emitter := setupCoordTest(t, "golang", "test-thread")

	chatObj := &thinkgraph.Chat{ID: "test-thread", SpaceID: "golang"}

	client := &coordMockModelClient{
		responses: []llm.StreamChunk{
			{
				ToolCalls: []llm.ToolCall{{
					Name: "propose_change",
					Args: map[string]any{
						tools.Instruction:   "Implement feature X",
						tools.ContextDigest: "Background context",
					},
				}},
			},
		},
	}

	flow := &coordMockFlow{}
	digestManager := session.NewDigestManager(logger, client, session.DigestConfig{})

	flowEngine := session.NewFlowEngine(logger, workspaceService, nil, workspaceService.Dispatcher(), emitter)

	registry := tools.NewRegistry()
	thinkSpace := &coordMockThinkSpace{}
	ui := &coordMockUI{strategyToReturn: flows.StrategyManual, reviewToReturn: true}

	coordinator := session.NewTurnCoordinator(logger, workspaceService, client, emitter, flowEngine, flow, digestManager, "golang", "", "", registry, thinkSpace, ui)
	registry.Register(tools.NewProposeChangeTool(coordinator))

	err := coordinator.RunAgentLoop(ctx, chatObj, activeSession, thinkSpace, cognition.ThinkBundle{})
	if err != nil {
		t.Fatalf("RunAgentLoop failed: %v", err)
	}

	if !flow.executeCalled {
		t.Errorf("Expected Flow to be evaluated by the Engine upon intercepting tool call")
	}
}

func TestTurnCoordinator_SynthesisLoop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	activeSession, workspaceService, emitter := setupCoordTest(t, "golang", "test-thread-synth")

	chatObj := &thinkgraph.Chat{ID: "test-thread-synth", SpaceID: "golang"}

	client := &coordMockModelClient{
		responses: []llm.StreamChunk{
			{
				ToolCalls: []llm.ToolCall{{
					Name: "propose_change",
					Args: map[string]any{
						tools.Instruction: "Original task",
					},
				}},
			},
			{
				Text: "I have reviewed the synthesis dossier and will now output the final Golden candidate.",
			},
		},
	}

	flow := &coordMockFlow{synthesizeOnExec: true}
	digestManager := session.NewDigestManager(logger, client, session.DigestConfig{})

	flowEngine := session.NewFlowEngine(logger, workspaceService, nil, workspaceService.Dispatcher(), emitter)

	registry := tools.NewRegistry()
	thinkSpace := &coordMockThinkSpace{}
	ui := &coordMockUI{strategyToReturn: flows.StrategyManual, reviewToReturn: true}

	coordinator := session.NewTurnCoordinator(logger, workspaceService, client, emitter, flowEngine, flow, digestManager, "golang", "", "", registry, thinkSpace, ui)
	registry.Register(tools.NewProposeChangeTool(coordinator))

	err := coordinator.RunAgentLoop(ctx, chatObj, activeSession, thinkSpace, cognition.ThinkBundle{
		Trajectory: cognition.Trajectory{
			WorkingMemory: []cognition.TrajectoryEvent{{Role: "user", Content: "Original setup turn"}},
		},
	})
	if err != nil {
		t.Fatalf("RunAgentLoop failed: %v", err)
	}

	// Verify the bundle was replaced with the focused dossier
	if len(client.capturedBundle.Trajectory.WorkingMemory) > 0 {
		t.Fatalf("Expected synthesis context to clear WorkingMemory, got %d items", len(client.capturedBundle.Trajectory.WorkingMemory))
	}

	lastMemory := client.capturedBundle.Trajectory.Catalyst
	if !strings.Contains(lastMemory.Content, "Refine instruction") {
		t.Errorf("Expected synthesis request trace to be injected into the Manager's Catalyst, got %s", lastMemory.Content)
	}
}

func TestTurnCoordinator_QueryLens(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	activeSession, workspaceService, emitter := setupCoordTest(t, "golang", "test-thread-lens")

	chatObj := &thinkgraph.Chat{ID: "test-thread-lens", SpaceID: "golang"}

	ev := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Historical DB context!")
	ev.Tags = []string{"database"}
	_ = workspaceService.Dispatcher().AppendEvent(ctx, chatObj, ev)

	client := &coordMockModelClient{
		responses: []llm.StreamChunk{
			{
				ToolCalls: []llm.ToolCall{{
					Name: "query_lens",
					Args: map[string]any{"tag": "database"},
				}},
			},
			{
				Text: "Final answer after querying.",
			},
		},
	}

	flowEngine := session.NewFlowEngine(logger, workspaceService, nil, workspaceService.Dispatcher(), emitter)
	digestManager := session.NewDigestManager(logger, client, session.DigestConfig{})

	registry := tools.NewRegistry()
	thinkSpace := &coordMockThinkSpace{}
	ui := &coordMockUI{reviewToReturn: true}

	coordinator := session.NewTurnCoordinator(logger, workspaceService, client, emitter, flowEngine, &coordMockFlow{}, digestManager, "golang", "", "", registry, thinkSpace, ui)
	registry.Register(tools.NewQueryLensTool(coordinator))

	_ = coordinator.RunAgentLoop(ctx, chatObj, activeSession, thinkSpace, cognition.ThinkBundle{})

	foundLensLog := false
	for _, e := range emitter.events {
		if e.event.Type == "log" && e.event.Message == "Searching memory for #database..." {
			foundLensLog = true
		}
	}
	if !foundLensLog {
		if !strings.Contains(client.capturedBundle.Trajectory.WorkingMemory[len(client.capturedBundle.Trajectory.WorkingMemory)-1].Content, "Historical DB context!") {
			t.Errorf("Expected query_lens to inject historical context")
		}
	}
}

func TestTurnCoordinator_CreateDigest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	activeSession, workspaceService, emitter := setupCoordTest(t, "golang", "test-thread-digest")

	chatObj := &thinkgraph.Chat{ID: "test-thread-digest", SpaceID: "golang"}

	client := &coordMockModelClient{
		responses: []llm.StreamChunk{
			{
				ToolCalls: []llm.ToolCall{{
					Name: "create_digest",
					Args: map[string]any{
						"topic_name":  "Memory Compression",
						"tags":        []any{"auth"},
						"instruction": "Summarize the auth discussion.",
					},
				}},
			},
			{
				Text: "Generated Manual Summary",
			},
		},
	}

	digestManager := session.NewDigestManager(logger, client, session.DigestConfig{
		ManualPrompt: "Instruction: {{.Instruction}}\nContext: {{.Context}}",
	})
	flowEngine := session.NewFlowEngine(logger, workspaceService, nil, workspaceService.Dispatcher(), emitter)

	registry := tools.NewRegistry()
	thinkSpace := &coordMockThinkSpace{}
	ui := &coordMockUI{reviewToReturn: true}

	coordinator := session.NewTurnCoordinator(logger, workspaceService, client, emitter, flowEngine, &coordMockFlow{}, digestManager, "golang", "", "", registry, thinkSpace, ui)
	registry.Register(tools.NewCreateDigestTool(coordinator))

	_ = coordinator.RunAgentLoop(ctx, chatObj, activeSession, thinkSpace, cognition.ThinkBundle{})

	digestsDir := filepath.Join(workspaceService.WorkspaceRoot(), "golang", "chats", chatObj.ID, "ledger", "digests")
	entries, err := os.ReadDir(digestsDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("Expected create_digest tool to route execution to the DigestManager and write to ledger digests dir")
	}

	content, err := os.ReadFile(filepath.Join(digestsDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("Failed to read digest file")
	}

	output := string(content)
	if !strings.Contains(output, "Generated Manual Summary") {
		t.Errorf("Expected digest body in markdown, got: %s", output)
	}
	if !strings.Contains(output, "auth") {
		t.Errorf("Expected 'auth' tag in markdown frontmatter, got: %s", output)
	}
}

func TestTurnCoordinator_TransitionWorkspace_Approved(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	activeSession, workspaceService, emitter := setupCoordTest(t, "concept", "test-thread-trans")

	chatObj := &thinkgraph.Chat{ID: "test-thread-trans", SpaceID: "concept"}

	client := &coordMockModelClient{
		responses: []llm.StreamChunk{
			{
				ToolCalls: []llm.ToolCall{{
					Name: "transition_workspace",
					Args: map[string]any{
						"target_think_state": "golang-space",
						"reasoning":          "Needs compilation",
					},
				}},
			},
		},
	}

	flowEngine := session.NewFlowEngine(logger, workspaceService, nil, workspaceService.Dispatcher(), emitter)
	digestManager := session.NewDigestManager(logger, client, session.DigestConfig{})

	registry := tools.NewRegistry()
	registry.Register(tools.NewTransitionThinkStateTool([]string{"golang-space"}))

	thinkSpace := &coordMockThinkSpace{}
	ui := &coordMockUI{transitionToReturn: true, reviewToReturn: true}

	coordinator := session.NewTurnCoordinator(logger, workspaceService, client, emitter, flowEngine, &coordMockFlow{}, digestManager, "concept", "", "", registry, thinkSpace, ui)

	err := coordinator.RunAgentLoop(ctx, chatObj, activeSession, thinkSpace, cognition.ThinkBundle{})

	if err == nil {
		t.Fatalf("Expected TransitionRequest error, got nil")
	}

	tr, ok := err.(*tools.TransitionThinkStateRequest)
	if !ok {
		t.Fatalf("Expected error of type *tools.TransitionThinkStateRequest, got %T: %v", err, err)
	}

	if tr.TargetThinkState != "golang-space" {
		t.Errorf("Expected transition to 'golang-space', got '%s'", tr.TargetThinkState)
	}

	if ui.capturedReasoning != "Needs compilation" {
		t.Errorf("Expected reasoning 'Needs compilation', got '%s'", ui.capturedReasoning)
	}
}
