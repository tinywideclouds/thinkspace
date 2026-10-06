package session_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

func TestDelegator_ExecuteProposeChange_Success(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	workspaceRoot := t.TempDir()
	bus := chat.NewEventBus()

	workspaceService := workspace.NewService(logger, &mockChatEngine{}, workspaceRoot, bus)
	thread, _ := workspaceService.StartThread(ctx, "test-thread")
	os.MkdirAll(filepath.Join(workspaceRoot, "chats", "test-thread"), 0755)

	client := &mockModelClient{}
	flow := &mockFlow{}
	emitter := &mockEmitter{}

	delegator := session.NewDelegator(
		logger, workspaceService, client, nil, flow, emitter, "golang", "Rules", flows.FlowConfig{},
	)

	userInterface := &mockUserInterface{strategyToReturn: session.StrategyManual, reviewToReturn: true}

	call := llm.ToolCall{
		Name: "propose_change",
		Args: map[string]any{
			"agent_tasks": []any{
				map[string]any{"instruction": "do test"},
			},
		},
	}

	space := &mockThinkSpace{}
	bundle := cognition.ThinkBundle{}

	err := delegator.ExecuteProposeChange(ctx, thread, space, bundle, userInterface, call, "test-model")
	if err != nil {
		t.Fatalf("ExecuteProposeChange failed: %v", err)
	}

	if !flow.executeCalled {
		t.Errorf("Expected FanOutFlow to be executed")
	}

	if userInterface.reviewedBranch != "candidate/mock-123" {
		t.Errorf("Expected branch 'candidate/mock-123' to be reviewed, got '%s'", userInterface.reviewedBranch)
	}
}

func TestDelegator_ExecuteProposeChange_AllFailed(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	workspaceService := workspace.NewService(logger, &mockChatEngine{}, t.TempDir(), chat.NewEventBus())
	thread, _ := workspaceService.StartThread(ctx, "test-thread")

	flow := &mockFlowFail{}
	delegator := session.NewDelegator(logger, workspaceService, &mockModelClient{}, nil, flow, &mockEmitter{}, "golang", "", flows.FlowConfig{})

	ui := &mockUserInterface{}
	err := delegator.ExecuteProposeChange(ctx, thread, &mockThinkSpace{}, cognition.ThinkBundle{}, ui, llm.ToolCall{}, "test-model")

	if err != nil {
		t.Fatalf("Did not expect an error for sub-agent test failures, got %v", err)
	}

	if ui.reviewedBranch != "" {
		t.Errorf("Expected no branch to reach manual review when all fail")
	}
}

type mockFlowFail struct{}

func (m *mockFlowFail) Name() string { return "FailFlow" }
func (m *mockFlowFail) Execute(ctx context.Context, service *workspace.Service, thread *chat.Thread, space spaces.ThinkSpace, args map[string]any, config flows.FlowConfig, flowCtx flows.FlowContext, emitter telemetry.Emitter, executor workspace.SubAgentExecutor, verifier spaces.Verifier) (*flows.FlowResult, error) {
	return &flows.FlowResult{
		Branches: []flows.BranchResult{{CandidateID: "fail-branch", Passed: false}},
	}, nil
}
