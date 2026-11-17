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

func (m *mockTransitionUI) ConfirmTransition(targetThinkState string, reasoning string) bool {
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

	signal, ok := result.Signal.(*signals.SignalTransition)
	if !ok {
		t.Fatalf("expected SignalTransition, got %T", result.Signal)
	}
	if signal.TargetThinkState != "golang" {
		t.Errorf("expected 'golang', got %q", signal.TargetThinkState)
	}
	if result.Content != "" {
		t.Errorf("expected completely silent transition content on the main timeline, got %q", result.Content)
	}
}

func TestTransitionThinkStateTool_Execute_Denied(t *testing.T) {
	tool := tools.NewTransitionThinkStateTool([]string{"golang", "concept"})
	emitter := &mockToolEmitter{}

	rawArgs := map[string]any{
		tools.TargetThinkState: "golang",
		tools.Reasoning:        "Need code",
	}

	bundle := &cognition.ThinkBundle{}

	ec := tools.ExecutionContext{
		RawArguments: rawArgs,
		UI:           &mockTransitionUI{confirmResult: false},
		Bundle:       bundle,
		Emitter:      emitter,
	}

	result, err := tool.Execute(context.Background(), ec)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Signal != nil {
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

	foundIntercept := false
	for _, e := range emitter.events {
		if e.Type == "system_intercept" {
			foundIntercept = true
		}
	}
	if !foundIntercept {
		t.Errorf("expected system_intercept telemetry event to be fired")
	}
}

func TestTransitionThinkStateTool_InvalidStateInterceptsForSelfCorrection(t *testing.T) {
	tool := tools.NewTransitionThinkStateTool([]string{"golang", "concept"})
	emitter := &mockToolEmitter{}
	bundle := &cognition.ThinkBundle{}

	result, err := tool.Execute(context.Background(), tools.ExecutionContext{
		RawArguments: map[string]any{
			tools.TargetThinkState: "rust",
			tools.Reasoning:        "Write Rust",
		},
		UI:      &mockTransitionUI{confirmResult: true},
		Bundle:  bundle,
		Emitter: emitter,
	})
	if err != nil {
		t.Fatalf("expected self-correction intercept rather than fatal error, got: %v", err)
	}
	if result.Signal != nil {
		t.Errorf("expected nil signal for unknown target state, got %T", result.Signal)
	}
	if len(bundle.Trajectory.WorkingMemory) != 1 || !strings.Contains(bundle.Trajectory.WorkingMemory[0].Content, "golang, concept") {
		t.Errorf("expected valid states guidance in WorkingMemory, got %+v", bundle.Trajectory.WorkingMemory)
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
	if result.Signal != nil {
		t.Errorf("expected nil signal on error")
	}
}
