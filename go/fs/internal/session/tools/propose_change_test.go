package tools_test

import (
	"context"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

type mockFanOutDispatcher struct {
	lastArgs    map[string]any
	lastSummary string
}

func (m *mockFanOutDispatcher) DispatchFanOut(ctx context.Context, chatObj *thinkgraph.Chat, bundle *cognition.ThinkBundle, args map[string]any, summary string) error {
	m.lastArgs = args
	m.lastSummary = summary
	return nil
}

func TestProposeChangeTool_Execute(t *testing.T) {
	dispatcher := &mockFanOutDispatcher{}
	tool := tools.NewProposeChangeTool(dispatcher)

	rawArgs := map[string]any{
		tools.AgentCount:    float64(2),
		tools.ContextDigest: "Testing context",
		tools.Instruction:   "Generate a base Go HTTP server",
		tools.TargetFiles:   []any{"src/main.go"},
	}

	ec := tools.ExecutionContext{
		ChatID:       "chat-123",
		SpaceID:      "space-123",
		RawArguments: rawArgs,
		Bundle:       &cognition.ThinkBundle{},
		UI:           &mockUI{},
	}

	shouldBreak, err := tool.Execute(context.Background(), ec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !shouldBreak {
		t.Errorf("expected tool to break the loop")
	}

	if dispatcher.lastArgs == nil {
		t.Fatalf("expected arguments to be passed to dispatcher")
	}

	tasks, ok := dispatcher.lastArgs["agent_tasks"].([]any)
	if !ok {
		t.Fatalf("expected agent_tasks array in flow args")
	}

	if len(tasks) != 2 {
		t.Errorf("expected 2 agent tasks mapped, got %d", len(tasks))
	}

	firstTask, ok := tasks[0].(map[string]any)
	if !ok {
		t.Fatalf("expected task to be a map")
	}

	if firstTask["instruction"] != "Generate a base Go HTTP server" {
		t.Errorf("expected instruction to map correctly, got %v", firstTask["instruction"])
	}

	targetFiles, ok := firstTask["target_files"].([]any)
	if !ok || len(targetFiles) != 1 || targetFiles[0] != "src/main.go" {
		t.Errorf("expected target_files to map correctly, got %v", firstTask["target_files"])
	}
}

func TestProposeChangeTool_EmptyInstruction(t *testing.T) {
	dispatcher := &mockFanOutDispatcher{}
	tool := tools.NewProposeChangeTool(dispatcher)

	rawArgs := map[string]any{
		tools.Instruction: "",
	}

	ec := tools.ExecutionContext{
		RawArguments: rawArgs,
		UI:           &mockUI{},
	}

	shouldBreak, err := tool.Execute(context.Background(), ec)
	if err == nil {
		t.Fatalf("expected error on empty instruction")
	}
	if !shouldBreak {
		t.Errorf("expected tool to break the loop on error")
	}
}
