package tools_test

import (
	"context"
	"strings"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
)

func TestProposeChangeTool_Execute_DistinctAgentInstructions(t *testing.T) {
	tool := tools.NewProposeChangeTool()
	emitter := &mockToolEmitter{}

	rawArgs := map[string]any{
		tools.Intention:     "Compare iterative vs recursive traversal performance",
		tools.AssignedTags:  []any{"Auth", "#middleware", "auth"},
		tools.AgentCount:    float64(2),
		tools.ContextDigest: "Testing context",
		tools.TargetFiles:   []any{"src/main.go"},
		tools.AgentInstructions: []any{
			map[string]any{
				tools.Approach:    "iterative",
				tools.Instruction: "Implement using an iterative loop",
			},
			map[string]any{
				tools.Approach:    "recursive",
				tools.Instruction: "Implement using tail recursion",
			},
		},
	}

	ec := tools.ExecutionContext{
		Address:      nil,
		RawArguments: rawArgs,
		Bundle:       &cognition.ThinkBundle{},
		UI:           &mockUI{},
		Emitter:      emitter,
	}

	result, err := tool.Execute(context.Background(), ec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	signal, ok := result.Signal.(*signals.SignalDispatchFlow)
	if !ok {
		t.Fatalf("expected SignalDispatchFlow, got %T", result.Signal)
	}

	if signal.Intention != "Compare iterative vs recursive traversal performance" {
		t.Errorf("expected SignalDispatchFlow.Intention to be preserved, got %q", signal.Intention)
	}
	if len(signal.Tags) != 2 || signal.Tags[0] != "auth" || signal.Tags[1] != "middleware" {
		t.Errorf("expected normalized deduplicated tags [auth middleware], got %v", signal.Tags)
	}

	if len(signal.Tasks) != 2 {
		t.Fatalf("expected 2 distinct agent tasks, got %d", len(signal.Tasks))
	}

	if signal.Tasks[0].AgentID != "agent-1" || !strings.Contains(signal.Tasks[0].Instruction, "iterative loop") {
		t.Errorf("expected agent-1 to receive iterative instruction, got %+v", signal.Tasks[0])
	}

	if signal.Tasks[1].AgentID != "agent-2" || !strings.Contains(signal.Tasks[1].Instruction, "tail recursion") {
		t.Errorf("expected agent-2 to receive recursive instruction, got %+v", signal.Tasks[1])
	}

	if signal.Tasks[0].Instruction == signal.Tasks[1].Instruction {
		t.Errorf("expected distinct instructions across parallel agents, got identical strings")
	}

	if len(signal.Tasks[0].TargetFiles) != 1 || signal.Tasks[0].TargetFiles[0] != "src/main.go" {
		t.Errorf("expected target_files to map correctly, got %v", signal.Tasks[0].TargetFiles)
	}

	if !strings.Contains(result.Content, "Executing 2 agent(s): Compare iterative vs recursive traversal performance") {
		t.Errorf("expected generated content to include Manager intention, got %q", result.Content)
	}

	foundDispatch := false
	for _, e := range emitter.events {
		if e.Type == "flow_dispatch" {
			foundDispatch = true
		}
	}
	if !foundDispatch {
		t.Errorf("expected flow_dispatch telemetry event to be fired")
	}
}

func TestProposeChangeTool_Execute_AgentInstructionsAuthoritative(t *testing.T) {
	tool := tools.NewProposeChangeTool()
	emitter := &mockToolEmitter{}

	// LLM passes agent_count: 2, but provides 1 concrete instruction object
	rawArgs := map[string]any{
		tools.AgentCount:    float64(2),
		tools.ContextDigest: "Testing context",
		tools.TargetFiles:   []any{"src/main.go"},
		tools.AgentInstructions: []any{
			map[string]any{
				tools.Approach:    "iterative",
				tools.Instruction: "Implement using an iterative loop",
			},
		},
	}

	ec := tools.ExecutionContext{
		Address:      nil,
		RawArguments: rawArgs,
		Bundle:       &cognition.ThinkBundle{},
		UI:           &mockUI{},
		Emitter:      emitter,
	}

	result, err := tool.Execute(context.Background(), ec)
	if err != nil {
		t.Fatalf("expected agent_instructions length to be authoritative without erroring, got: %v", err)
	}

	signal, ok := result.Signal.(*signals.SignalDispatchFlow)
	if !ok {
		t.Fatalf("expected SignalDispatchFlow, got %T", result.Signal)
	}

	if len(signal.Tasks) != 1 {
		t.Fatalf("expected 1 task matching len(agent_instructions), got %d", len(signal.Tasks))
	}

	if !strings.Contains(result.Content, "Executing 1 agent(s)") {
		t.Errorf("expected summary to reflect 1 agent, got %q", result.Content)
	}
}

func TestProposeChangeTool_EmptyInstruction(t *testing.T) {
	tool := tools.NewProposeChangeTool()
	emitter := &mockToolEmitter{}

	rawArgs := map[string]any{
		tools.Instruction: "",
	}

	ec := tools.ExecutionContext{
		RawArguments: rawArgs,
		UI:           &mockUI{},
		Emitter:      emitter,
	}

	result, err := tool.Execute(context.Background(), ec)
	if err == nil {
		t.Fatalf("expected error on empty instruction")
	}
	if result.Signal != nil {
		t.Errorf("expected nil signal on error")
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
