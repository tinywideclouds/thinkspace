package session_test

import (
	"context"
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
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"uuid"
)

type teTestModelClient struct {
	responses      []llm.StreamChunk
	callIndex      int
	capturedBundle cognition.ThinkBundle
}

func (client *teTestModelClient) GenerateContentStream(
	ctx context.Context,
	model string,
	bundle cognition.ThinkBundle,
	registry *tools.Registry,
	config spaces.ThinkStateConfig,
) iter.Seq2[llm.StreamChunk, error] {
	client.capturedBundle = bundle
	return func(yield func(llm.StreamChunk, error) bool) {
		for client.callIndex < len(client.responses) {
			resp := client.responses[client.callIndex]
			client.callIndex++
			if !yield(resp, nil) {
				return
			}
		}
	}
}

type teTestChatEngine struct{}

func (engine *teTestChatEngine) InitChat(ctx context.Context, chatID string) error { return nil }
func (engine *teTestChatEngine) Snapshot(ctx context.Context, chatID, message string) (string, error) {
	return "mock-sha", nil
}
func (engine *teTestChatEngine) SpawnCandidateSandbox(ctx context.Context, chatID, candidateID string) (workspace.CandidateSandbox, error) {
	return nil, nil
}
func (engine *teTestChatEngine) PreviewCandidate(ctx context.Context, chatID, candidateID string) error {
	return nil
}
func (engine *teTestChatEngine) Accept(ctx context.Context, chatID, candidateID, reason string) error {
	return nil
}
func (engine *teTestChatEngine) Reject(ctx context.Context, chatID, candidateID, reason string) error {
	return nil
}
func (engine *teTestChatEngine) ReadCandidateDiff(ctx context.Context, chatID, candidateID string) (string, error) {
	return "+ mock diff", nil
}

type teTestThinkSpace struct{}

func (space *teTestThinkSpace) Config() spaces.ThinkStateConfig {
	cfg := spaces.ThinkStateConfig{
		Roles: spaces.RolesConfig{
			Manager: spaces.ManagerConfig{Model: "test-manager"},
		},
		Timeouts: spaces.TimeoutsConfig{TurnSeconds: 300},
	}
	cfg.ApplyDefaults()
	return cfg
}
func (space *teTestThinkSpace) Verifier() spaces.Verifier { return nil }
func (space *teTestThinkSpace) Mapbook() spaces.Mapbook   { return space }
func (space *teTestThinkSpace) GenerateLayers(ctx context.Context, workspaceRoot string) (map[spaces.MapbookLayer]string, error) {
	return map[spaces.MapbookLayer]string{}, nil
}
func (space *teTestThinkSpace) Patcher() workspace.Patcher                             { return nil }
func (space *teTestThinkSpace) Initialize(ctx context.Context, directory string) error { return nil }
func (space *teTestThinkSpace) EnrichBundle(ctx context.Context, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus, workspaceRoot string) error {
	return nil
}

type teTestUI struct {
	streamedNodes []thinkgraph.Node
	chunks        []workspace.NodeChunk
}

func (ui *teTestUI) OnTextChunk(chunk workspace.NodeChunk) {
	ui.chunks = append(ui.chunks, chunk)
}
func (ui *teTestUI) StreamGraphEvent(address *thinkgraph.ChatAddress, node thinkgraph.Node) {
	ui.streamedNodes = append(ui.streamedNodes, node)
}
func (ui *teTestUI) ConfirmTransition(req signals.SignalTransition) bool { return true }
func (ui *teTestUI) ChooseNextStep() flows.FlowStrategy                  { return flows.StrategyManual }
func (ui *teTestUI) ReviewCandidate(branch string) bool                  { return true }
func (ui *teTestUI) ReviewContext(ctxData session.TurnContext) session.ContextApprovalDecision {
	return session.ContextApprovalDecision{}
}
func (ui *teTestUI) GetChunkChannel() chan<- workspace.NodeChunk { return nil }

type teTestEmitter struct{}

func (emitter *teTestEmitter) Emit(channel string, event telemetry.Event) {}

type teMockSynthesisTool struct{}

func (t *teMockSynthesisTool) Name() tools.ToolName       { return "mock_synth_tool" }
func (t *teMockSynthesisTool) DefaultDescription() string { return "" }
func (t *teMockSynthesisTool) Execute(ctx context.Context, ec tools.ExecutionContext) (signals.EngineSignal, error) {
	return &signals.SignalSynthesize{OriginalInstruction: "Dossier output"}, nil
}

func setupExecutorTest(t *testing.T, spaceID, chatID string) (*session.TurnExecutor, *workspace.Service, *teTestModelClient) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, _ := db.NewBadgerGraph(opts)
	t.Cleanup(func() { graphDB.Close() })

	emitter := &teTestEmitter{}
	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatVersioner { return &teTestChatEngine{} }, graphDB, emitter)
	_ = os.MkdirAll(filepath.Join(workspaceRoot, spaceID), 0755)

	client := &teTestModelClient{}
	workspaceService := svcManager.GetService(spaceID)

	executor := session.NewTurnExecutor(logger, client, workspaceService, emitter)

	return executor, workspaceService, client
}

func TestTurnExecutor_Execute_StandardResponse_OpenNodeHandshake(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	executor, wsService, client := setupExecutorTest(t, "golang", "test-thread")
	responseID := uuid.NewV7()
	parentID := uuid.NewV7()

	address := &thinkgraph.ChatAddress{
		ChatID:    "test-thread",
		ProjectID: "golang",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}

	client.responses = []llm.StreamChunk{
		{Text: "Hello "},
		{Text: "world"},
	}

	registry := tools.NewRegistry()
	ui := &teTestUI{}

	signal, err := executor.Execute(ctx, address, responseID, parentID, true, &teTestThinkSpace{}, cognition.ThinkBundle{}, registry, ui)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if _, ok := signal.(*signals.SignalTerminal); !ok {
		t.Errorf("Expected SignalTerminal for standard response, got %T", signal)
	}

	if len(ui.streamedNodes) != 2 {
		t.Fatalf("Expected exactly 2 node streams (Shell Open, Final Lock), got %d", len(ui.streamedNodes))
	}

	shellNode := ui.streamedNodes[0]
	if shellNode.Content != "" {
		t.Errorf("Expected first streamed node to be an empty shell, got content: %s", shellNode.Content)
	}
	if shellNode.Edges["parent_id"] != parentID.String() {
		t.Errorf("Expected shell node to carry parent_id edge %s, got %v", parentID.String(), shellNode.Edges)
	}

	finalNode := ui.streamedNodes[1]
	if finalNode.ID != shellNode.ID {
		t.Errorf("Expected final node ID to match shell node ID for UI locking")
	}
	if finalNode.Content != "Hello world" {
		t.Errorf("Expected final node to contain complete content, got %s", finalNode.Content)
	}
	if finalNode.Edges["parent_id"] != parentID.String() {
		t.Errorf("Expected final node to carry parent_id edge %s, got %v", parentID.String(), finalNode.Edges)
	}

	if len(ui.chunks) != 2 {
		t.Fatalf("Expected exactly 2 text chunks streamed, got %d", len(ui.chunks))
	}
	if ui.chunks[0].NodeID != shellNode.ID || ui.chunks[1].NodeID != shellNode.ID {
		t.Errorf("Expected text chunks to be attached to the pre-allocated node ID")
	}

	timeline, _ := wsService.Graph().GetSubGraphTimeline(ctx, address, address.GraphID)
	if len(timeline) == 0 {
		t.Errorf("Expected standard generation to be appended to graph timeline")
	} else if timeline[len(timeline)-1].ID != shellNode.ID {
		t.Errorf("Expected persisted node ID to match the pre-allocated ID")
	}
}

func TestTurnExecutor_Execute_RoutesToTool(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	executor, wsService, client := setupExecutorTest(t, "golang", "test-thread")
	responseID := uuid.NewV7()

	address := &thinkgraph.ChatAddress{
		ChatID:    "test-thread",
		ProjectID: "golang",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}

	client.responses = []llm.StreamChunk{
		{
			ToolCalls: []llm.ToolCall{{
				Name: "transition_workspace",
				Args: map[string]any{
					"target_think_state": "concept",
					"reasoning":          "Time to plan",
				},
			}},
		},
	}

	registry := tools.NewRegistry()
	registry.Register(tools.NewTransitionThinkStateTool([]string{"concept"}))

	signal, err := executor.Execute(ctx, address, responseID, uuid.Nil(), false, &teTestThinkSpace{}, cognition.ThinkBundle{}, registry, &teTestUI{})
	if err != nil {
		t.Fatalf("Expected nil error, got %v", err)
	}

	tr, ok := signal.(*signals.SignalTransition)
	if !ok {
		t.Fatalf("Expected SignalTransition, got %T", signal)
	}
	if tr.TargetThinkState != "concept" {
		t.Errorf("Expected transition to concept, got %s", tr.TargetThinkState)
	}

	timeline, _ := wsService.Graph().GetSubGraphTimeline(ctx, address, address.GraphID)
	if len(timeline) != 0 {
		t.Errorf("Expected zero persisted nodes in main timeline on direct tool intercept, got %d", len(timeline))
	}
}

func TestTurnExecutor_Execute_SynthesisLoop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	executor, _, client := setupExecutorTest(t, "golang", "test-thread")
	responseID := uuid.NewV7()

	address := &thinkgraph.ChatAddress{
		ChatID:    "test-thread",
		ProjectID: "golang",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}

	client.responses = []llm.StreamChunk{
		{
			ToolCalls: []llm.ToolCall{{Name: "mock_synth_tool"}},
		},
		{
			Text: "Synthesized golden output",
		},
	}

	registry := tools.NewRegistry()
	registry.Register(&teMockSynthesisTool{})

	initialBundle := cognition.ThinkBundle{
		Trajectory: cognition.Trajectory{
			WorkingMemory: []cognition.TrajectoryEvent{{Role: "user", Content: "Initial memory"}},
		},
	}

	signal, err := executor.Execute(ctx, address, responseID, uuid.Nil(), false, &teTestThinkSpace{}, initialBundle, registry, &teTestUI{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	synthSig, ok := signal.(*signals.SignalSynthesize)
	if !ok {
		t.Fatalf("Expected SignalSynthesize, got %T", signal)
	}

	if synthSig.OriginalInstruction != "Dossier output" {
		t.Errorf("Expected Dossier output, got %s", synthSig.OriginalInstruction)
	}
}
