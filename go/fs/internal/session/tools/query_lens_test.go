package tools_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

type mockWorkspace struct {
	graph thinkgraph.GraphRepository
}

func (m *mockWorkspace) Graph() thinkgraph.GraphRepository { return m.graph }
func (m *mockWorkspace) WorkspaceRoot() string             { return "/mock" }

type mockGraphRepo struct {
	thinkgraph.GraphRepository
}

func (m *mockGraphRepo) GetLenses(ctx context.Context, c *thinkgraph.ChatAddress, tag string) (thinkgraph.LensResult, error) {
	return thinkgraph.LensResult{
		Nodes: []thinkgraph.Node{
			{
				Event: thinkgraph.Event{
					Type:      thinkgraph.EventPrompt,
					Content:   "Historical context about " + tag,
					Timestamp: time.Now(),
				},
			},
		},
	}, nil
}

func TestQueryLensTool_Execute(t *testing.T) {
	ws := &mockWorkspace{graph: &mockGraphRepo{}}
	tool := tools.NewQueryLensTool(ws)

	rawArgs := map[string]any{
		tools.Tag:       "security",
		tools.Reasoning: "Need to check auth history",
	}

	bundle := &cognition.ThinkBundle{}

	ec := tools.ExecutionContext{
		RawArguments: rawArgs,
		Bundle:       bundle,
		UI:           &mockUI{},
	}

	result, err := tool.Execute(context.Background(), ec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected tool to return a nil signal (continue turn)")
	}

	if len(bundle.Trajectory.WorkingMemory) != 1 {
		t.Fatalf("expected 1 event injected into working memory, got %d", len(bundle.Trajectory.WorkingMemory))
	}

	injected := bundle.Trajectory.WorkingMemory[0]
	if injected.Role != "user" {
		t.Errorf("expected role 'user', got %q", injected.Role)
	}
	if !strings.Contains(injected.Content, "Historical context about security") {
		t.Errorf("expected tool response to contain queried history, got: %s", injected.Content)
	}
}
