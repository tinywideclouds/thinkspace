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
		if client.callIndex < len(client.responses) {
			resp := client.responses[client.callIndex]
			client.callIndex++
			yield(resp, nil)
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
func (space *teTestThinkSpace) Verifier() spaces.Verifier                              { return nil }
func (space *teTestThinkSpace) Mapbook() spaces.Mapbook                                { return nil }
func (space *teTestThinkSpace) Patcher() workspace.Patcher                             { return nil }
func (space *teTestThinkSpace) Initialize(ctx context.Context, directory string) error { return nil }
func (space *teTestThinkSpace) EnrichBundle(ctx context.Context, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus, workspaceRoot string) error {
	return nil
}

type teTestUI struct{}

func (ui *teTestUI) OnTextChunk(text string)                               {}
func (ui *teTestUI) ConfirmTransition(targetDomain, reasoning string) bool { return true }
func (ui *teTestUI) ChooseNextStep() flows.FlowStrategy                    { return flows.StrategyManual }
func (ui *teTestUI) ReviewCandidate(branch string) bool                    { return true }
func (ui *teTestUI) ReviewContext(ctxData session.TurnContext) session.ContextApprovalDecision {
	return session.ContextApprovalDecision{}
}
func (ui *teTestUI) GetAgentTokenChannel() chan<- workspace.AgentToken { return nil }

type teTestEmitter struct{}

func (emitter *teTestEmitter) Emit(channel string, event telemetry.Event) {}

// A mock tool that returns a SynthesisRequest to test the bouncing behavior
type teMockSynthesisTool struct{}

func (t *teMockSynthesisTool) Name() tools.ToolName       { return "mock_synth_tool" }
func (t *teMockSynthesisTool) DefaultDescription() string { return "" }
func (t *teMockSynthesisTool) Execute(ctx context.Context, ec tools.ExecutionContext) (bool, error) {
	return false, &flows.SynthesisRequest{CompiledPrompt: "Dossier output"}
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

func TestTurnExecutor_Execute_StandardResponse(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	executor, wsService, client := setupExecutorTest(t, "golang", "test-thread")
	address := &thinkgraph.ChatAddress{ChatID: "test-thread", ProjectID: "golang"}

	client.responses = []llm.StreamChunk{{Text: "Hello world"}}

	registry := tools.NewRegistry()
	err := executor.Execute(ctx, address, &teTestThinkSpace{}, cognition.ThinkBundle{}, registry, &teTestUI{})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify it wrote the standard response to the exact ChatAddress
	timeline, _ := wsService.Graph().GetSubGraphTimeline(ctx, address, address.GraphID)
	if len(timeline) == 0 {
		t.Errorf("Expected standard generation to be appended to graph timeline")
	}
}

func TestTurnExecutor_Execute_RoutesToTool(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	executor, _, client := setupExecutorTest(t, "golang", "test-thread")
	address := &thinkgraph.ChatAddress{ChatID: "test-thread", ProjectID: "golang"}

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

	err := executor.Execute(ctx, address, &teTestThinkSpace{}, cognition.ThinkBundle{}, registry, &teTestUI{})
	if err == nil {
		t.Fatalf("Expected tool error (TransitionRequest), got nil")
	}

	tr, ok := err.(*tools.TransitionThinkStateRequest)
	if !ok {
		t.Errorf("Expected TransitionThinkStateRequest error, got %v", err)
	}
	if tr.TargetThinkState != "concept" {
		t.Errorf("Expected transition to concept, got %s", tr.TargetThinkState)
	}
}

func TestTurnExecutor_Execute_SynthesisLoop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	executor, _, client := setupExecutorTest(t, "golang", "test-thread")
	address := &thinkgraph.ChatAddress{ChatID: "test-thread", ProjectID: "golang"}

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

	err := executor.Execute(ctx, address, &teTestThinkSpace{}, initialBundle, registry, &teTestUI{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify the bundle was replaced with the focused dossier
	if len(client.capturedBundle.Trajectory.WorkingMemory) > 0 {
		t.Fatalf("Expected synthesis context to clear WorkingMemory, got %d items", len(client.capturedBundle.Trajectory.WorkingMemory))
	}

	lastMemory := client.capturedBundle.Trajectory.Catalyst
	if !strings.Contains(lastMemory.Content, "Dossier output") {
		t.Errorf("Expected synthesis request trace to be injected into the Catalyst, got %s", lastMemory.Content)
	}
}
