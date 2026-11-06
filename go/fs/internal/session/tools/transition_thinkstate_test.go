package tools_test

import (
	"context"
	"strings"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
)

type mockTransitionUI struct {
	mockUI
	confirmResult bool
}

func (m *mockTransitionUI) ConfirmTransition(req signals.SignalTransition) bool {
	return m.confirmResult
}

func TestTransitionThinkStateTool_Execute_Approved(t *testing.T) {
	tool := tools.NewTransitionThinkStateTool([]string{"golang", "concept"})

	rawArgs := map[string]any{
		tools.TargetThinkState: "golang",
		tools.Reasoning:        "Need code",
	}

	ec := tools.ExecutionContext{
		RawArguments: rawArgs,
		UI:           &mockTransitionUI{confirmResult: true},
	}

	result, err := tool.Execute(context.Background(), ec)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	signal, ok := result.(*signals.SignalTransition)
	if !ok {
		t.Fatalf("expected SignalTransition, got %T", result)
	}
	if signal.TargetThinkState != "golang" {
		t.Errorf("expected 'golang', got %q", signal.TargetThinkState)
	}
}

func TestTransitionThinkStateTool_Execute_Denied(t *testing.T) {
	tool := tools.NewTransitionThinkStateTool([]string{"golang", "concept"})

	rawArgs := map[string]any{
		tools.TargetThinkState: "golang",
		tools.Reasoning:        "Need code",
	}

	bundle := &cognition.ThinkBundle{}

	ec := tools.ExecutionContext{
		RawArguments: rawArgs,
		UI:           &mockTransitionUI{confirmResult: false},
		Bundle:       bundle,
	}

	result, err := tool.Execute(context.Background(), ec)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil signal on denied transition (LLM must self-correct)")
	}

	if len(bundle.Trajectory.WorkingMemory) != 1 {
		t.Fatalf("expected injection of system rejection message")
	}

	injected := bundle.Trajectory.WorkingMemory[0]
	if injected.Role != "user" {
		t.Errorf("expected role 'user', got %q", injected.Role)
	}
	if !strings.Contains(injected.Content, "denied by user") {
		t.Errorf("expected denial message, got %q", injected.Content)
	}
}

func TestTransitionThinkStateTool_EmptyTarget(t *testing.T) {
	tool := tools.NewTransitionThinkStateTool([]string{"golang", "concept"})

	rawArgs := map[string]any{
		tools.TargetThinkState: "",
		tools.Reasoning:        "Empty test",
	}

	ec := tools.ExecutionContext{
		RawArguments: rawArgs,
	}

	result, err := tool.Execute(context.Background(), ec)

	if err == nil {
		t.Fatalf("expected error on empty target")
	}
	if result != nil {
		t.Errorf("expected nil signal on error")
	}
}
