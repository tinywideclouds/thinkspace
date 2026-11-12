package thinkgraph_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

type mockGraphRepoForTraversal struct {
	thinkgraph.GraphRepository
	nodesByBranch map[string][]thinkgraph.Node
}

func (m *mockGraphRepoForTraversal) GetSubGraphTimelinePage(ctx context.Context, c *thinkgraph.ChatAddress, graphID thinkgraph.GraphID, cursor string, limit int) ([]thinkgraph.Node, string, bool, error) {
	branchNodes, ok := m.nodesByBranch[c.Name]
	if !ok {
		return nil, "", false, nil
	}

	startIdx := len(branchNodes)
	if cursor != "" {
		for i, n := range branchNodes {
			if n.ID.String() == cursor {
				startIdx = i + 1 // Inclusive of cursor when hopping across branches
				break
			}
		}
	}

	endIdx := startIdx - limit
	hasMore := true
	if endIdx <= 0 {
		endIdx = 0
		hasMore = false
	}

	page := branchNodes[endIdx:startIdx]
	var nextCursor string
	if hasMore && endIdx > 0 {
		nextCursor = branchNodes[endIdx-1].ID.String()
	}

	return page, nextCursor, hasMore, nil
}

func (m *mockGraphRepoForTraversal) WalkSubGraphReverse(ctx context.Context, c *thinkgraph.ChatAddress, graphID thinkgraph.GraphID, startCursor string, fn func(thinkgraph.Node) bool) error {
	branchNodes, ok := m.nodesByBranch[c.Name]
	if !ok {
		return nil
	}

	startIdx := len(branchNodes) - 1
	if startCursor != "" {
		for i, n := range branchNodes {
			if n.ID.String() == startCursor {
				startIdx = i // Inclusive of cursor when hopping across branches
				break
			}
		}
	}

	for i := startIdx; i >= 0; i-- {
		if !fn(branchNodes[i]) {
			break
		}
	}
	return nil
}

func TestTraversalEngine_ResolveSubGraphID(t *testing.T) {
	ctx := context.Background()

	lifecycleEvent := thinkgraph.NewEvent("flow_lifecycle", "Starting Flow")
	lifecycleEvent.Data = []byte(`{"flow_id":"flow-xyz-123"}`)

	mainNodes := []thinkgraph.Node{
		{Event: thinkgraph.NewEvent(thinkgraph.EventPrompt, "main-start")},
		{Event: lifecycleEvent},
		{Event: thinkgraph.NewEvent(thinkgraph.EventPrompt, "main-end")},
	}

	repo := &mockGraphRepoForTraversal{
		nodesByBranch: map[string][]thinkgraph.Node{
			"main": mainNodes,
		},
	}
	engine := thinkgraph.NewTraversalEngine(repo)

	chat := &thinkgraph.ChatAddress{
		ChatID:    "test-chat",
		ProjectID: "space",
		SubGraph:  thinkgraph.SubGraph{Name: "main"},
	}

	t.Run("Successfully resolves existing subgraph", func(t *testing.T) {
		graphID, err := engine.ResolveSubGraphID(ctx, chat, "flow-xyz-123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if graphID.UUID != lifecycleEvent.ID {
			t.Errorf("Expected UUID %v, got %v", lifecycleEvent.ID, graphID.UUID)
		}
		if graphID.Prefix != thinkgraph.PrefixFlow {
			t.Errorf("Expected Flow prefix, got %v", graphID.Prefix)
		}
	})

	t.Run("Fails cleanly on missing subgraph alias", func(t *testing.T) {
		graphID, err := engine.ResolveSubGraphID(ctx, chat, "flow-not-real")
		if err == nil {
			t.Errorf("Expected error for missing subgraph, got nil")
		}
		if !graphID.IsZero() {
			t.Errorf("Expected zeroed GraphID on error")
		}
	})
}

func TestTraversalEngine_GetTimelinePage(t *testing.T) {
	ctx := context.Background()

	mainNodes := make([]thinkgraph.Node, 3)
	for i := 0; i < 3; i++ {
		ev := thinkgraph.NewEvent(thinkgraph.EventPrompt, fmt.Sprintf("main-%d", i))
		ev.Timestamp = time.Now().Add(time.Duration(i) * time.Second)
		mainNodes[i] = thinkgraph.Node{Event: ev}
	}

	branchANodes := make([]thinkgraph.Node, 3)

	anchorEv := thinkgraph.NewEvent(thinkgraph.EventBranchAnchor, "Forked")
	anchorEv.Timestamp = time.Now().Add(10 * time.Second)
	anchorData := thinkgraph.BranchAnchor{
		ParentBranch: "main",
		ForkPoint:    mainNodes[2].ID,
	}
	anchorEv.Data, _ = json.Marshal(anchorData)
	branchANodes[0] = thinkgraph.Node{Event: anchorEv}

	for i := 1; i < 3; i++ {
		ev := thinkgraph.NewEvent(thinkgraph.EventPrompt, fmt.Sprintf("branchA-%d", i-1))
		ev.Timestamp = time.Now().Add(time.Duration(i+10) * time.Second)
		branchANodes[i] = thinkgraph.Node{Event: ev}
	}

	repo := &mockGraphRepoForTraversal{
		nodesByBranch: map[string][]thinkgraph.Node{
			"main":    mainNodes,
			"branchA": branchANodes,
		},
	}
	engine := thinkgraph.NewTraversalEngine(repo)

	chat := &thinkgraph.ChatAddress{
		ChatID:    "test-chat",
		ProjectID: "space",
		SubGraph: thinkgraph.SubGraph{
			Name: "branchA",
		},
	}

	page, nextCursor, hasMore, err := engine.GetTimelinePage(ctx, chat, "", 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(page) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(page))
	}

	if page[3].Content != "branchA-1" || page[2].Content != "branchA-0" || page[1].Type != thinkgraph.EventBranchAnchor || page[0].Content != "main-2" {
		t.Errorf("Unexpected timeline stitching order across branches")
	}

	if !hasMore {
		t.Errorf("Expected hasMore to be true because we haven't exhausted Main")
	}
	if nextCursor != mainNodes[1].ID.String() {
		t.Errorf("Expected nextCursor to be main-1, got %s", nextCursor)
	}
}

func TestTraversalEngine_GetFullTimeline(t *testing.T) {
	ctx := context.Background()

	mainNodes := []thinkgraph.Node{{Event: thinkgraph.NewEvent(thinkgraph.EventPrompt, "main-start")}}

	anchorEv := thinkgraph.NewEvent(thinkgraph.EventBranchAnchor, "Forked")
	anchorData := thinkgraph.BranchAnchor{
		ParentBranch: "main",
		ForkPoint:    mainNodes[0].ID,
	}
	anchorEv.Data, _ = json.Marshal(anchorData)

	branchNodes := []thinkgraph.Node{
		{Event: anchorEv},
		{Event: thinkgraph.NewEvent(thinkgraph.EventPrompt, "branch-1")},
	}

	repo := &mockGraphRepoForTraversal{
		nodesByBranch: map[string][]thinkgraph.Node{
			"main":    mainNodes,
			"branchA": branchNodes,
		},
	}
	engine := thinkgraph.NewTraversalEngine(repo)

	chat := &thinkgraph.ChatAddress{
		ChatID:    "test-chat",
		ProjectID: "space",
		SubGraph:  thinkgraph.SubGraph{Name: "branchA"},
	}

	timeline, err := engine.GetFullTimeline(ctx, chat)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(timeline) != 3 {
		t.Fatalf("expected 3 total nodes stitched together, got %d", len(timeline))
	}
	if timeline[0].Content != "main-start" || timeline[1].Type != thinkgraph.EventBranchAnchor || timeline[2].Content != "branch-1" {
		t.Errorf("Timeline returned in wrong chronological order")
	}
}
