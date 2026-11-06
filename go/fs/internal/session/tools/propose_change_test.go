package tools_test

import (
	"context"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
)

func TestProposeChangeTool_Execute(t *testing.T) {
	// The tool is now completely pure, no dispatcher needed
	tool := tools.NewProposeChangeTool()

	rawArgs := map[string]any{
		tools.AgentCount:    float64(2),
		tools.ContextDigest: "Testing context",
		tools.Instruction:   "Generate a base Go HTTP server",
		tools.TargetFiles:   []any{"src/main.go"},
	}

	ec := tools.ExecutionContext{
		Address:      nil,
		RawArguments: rawArgs,
		Bundle:       &cognition.ThinkBundle{},
		UI:           &mockUI{},
	}

	result, err := tool.Execute(context.Background(), ec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	signal, ok := result.(*signals.SignalDispatchFlow)
	if !ok {
		t.Fatalf("expected SignalDispatchFlow, got %T", result)
	}

	if len(signal.Tasks) != 2 {
		t.Errorf("expected 2 agent tasks, got %d", len(signal.Tasks))
	}

	if signal.Tasks[0].Instruction != "Generate a base Go HTTP server" {
		t.Errorf("expected instruction to map correctly, got %v", signal.Tasks[0].Instruction)
	}

	if len(signal.Tasks[0].TargetFiles) != 1 || signal.Tasks[0].TargetFiles[0] != "src/main.go" {
		t.Errorf("expected target_files to map correctly, got %v", signal.Tasks[0].TargetFiles)
	}
}

func TestProposeChangeTool_EmptyInstruction(t *testing.T) {
	tool := tools.NewProposeChangeTool()

	rawArgs := map[string]any{
		tools.Instruction: "",
	}

	ec := tools.ExecutionContext{
		RawArguments: rawArgs,
		UI:           &mockUI{},
	}

	result, err := tool.Execute(context.Background(), ec)
	if err == nil {
		t.Fatalf("expected error on empty instruction")
	}
	if result != nil {
		t.Errorf("expected nil signal on error")
	}
}
