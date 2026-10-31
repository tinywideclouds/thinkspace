package thinkgraph_test

import (
	"context"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

func TestDefaultBuilder_Build(t *testing.T) {
	ctx := context.Background()
	builder := &thinkgraph.DefaultBuilder{}
	chat := &thinkgraph.Chat{ID: "test-chat"}

	ev := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Test Lean Event")

	node, err := builder.Build(ctx, chat, ev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if node.ID != ev.ID {
		t.Errorf("expected ID %s, got %s", ev.ID, node.ID)
	}
	if len(node.MaterializedState) != 0 {
		t.Errorf("expected empty materialized state for default builder, got %s", string(node.MaterializedState))
	}
	if node.Tombstoned {
		t.Errorf("expected node to not be tombstoned by default")
	}
}
