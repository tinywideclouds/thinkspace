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
	digests []thinkgraph.DigestMeta
}

func (m *mockGraphRepo) GetLenses(ctx context.Context, c *thinkgraph.ChatAddress, tag string) (thinkgraph.LensResult, error) {
	if tag == "digest-only" {
		return thinkgraph.LensResult{Digests: m.digests}, nil
	}
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
	ws := &mockWorkspace{
		graph: &mockGraphRepo{
			digests: []thinkgraph.DigestMeta{
				{Summary: "Archived security digest"},
			},
		},
	}
	tool := tools.NewQueryLensTool(ws)
	emitter := &mockToolEmitter{}

	rawArgs := map[string]any{
		tools.Tag:       "security",
		tools.Reasoning: "Need to check auth history",
	}

	bundle := &cognition.ThinkBundle{}

	ec := tools.ExecutionContext{
		RawArguments: rawArgs,
		Bundle:       bundle,
		UI:           &mockUI{},
		Emitter:      emitter,
	}

	result, err := tool.Execute(context.Background(), ec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Signal != nil {
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

	foundStatus := false
	for _, e := range emitter.events {
		if e.Type == "tool_status" {
			foundStatus = true
		}
	}
	if !foundStatus {
		t.Errorf("expected tool_status telemetry event to be fired")
	}

	// Verify digest-only lens hits also render into WorkingMemory
	digestBundle := &cognition.ThinkBundle{}
	_, err = tool.Execute(context.Background(), tools.ExecutionContext{
		RawArguments: map[string]any{tools.Tag: "digest-only"},
		Bundle:       digestBundle,
		UI:           &mockUI{},
	})
	if err != nil {
		t.Fatalf("digest-only query_lens failed: %v", err)
	}
	if len(digestBundle.Trajectory.WorkingMemory) != 1 || !strings.Contains(digestBundle.Trajectory.WorkingMemory[0].Content, "Archived security digest") {
		t.Errorf("expected digest summary in WorkingMemory, got %+v", digestBundle.Trajectory.WorkingMemory)
	}

	// Empty tag must fail validation
	if _, err := tool.Execute(context.Background(), tools.ExecutionContext{RawArguments: map[string]any{tools.Tag: "  "}}); err == nil {
		t.Errorf("expected error for empty query_lens tag")
	}
}
