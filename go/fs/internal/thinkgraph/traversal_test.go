package thinkgraph_test

import (
	"context"
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
	// The mock fakes GraphID resolution by just reading c.Name for simplicity of testing traversal logic
	branchNodes, ok := m.nodesByBranch[c.Name]
	if !ok {
		return nil, "", false, nil
	}

	startIdx := len(branchNodes)
	if cursor != "" {
		for i, n := range branchNodes {
			if n.ID.String() == cursor {
				startIdx = i
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
				startIdx = i - 1
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

func TestTraversalEngine_GetTimelinePage(t *testing.T) {
	ctx := context.Background()

	// Create a chain of events: Main -> Branch A -> Branch B
	mainNodes := make([]thinkgraph.Node, 3)
	for i := 0; i < 3; i++ {
		ev := thinkgraph.NewEvent(thinkgraph.EventPrompt, fmt.Sprintf("main-%d", i))
		ev.Timestamp = time.Now().Add(time.Duration(i) * time.Second)
		mainNodes[i] = thinkgraph.Node{Event: ev}
	}

	branchANodes := make([]thinkgraph.Node, 2)
	for i := 0; i < 2; i++ {
		ev := thinkgraph.NewEvent(thinkgraph.EventPrompt, fmt.Sprintf("branchA-%d", i))
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

	topology := map[string]thinkgraph.BranchTopology{
		"branchA": {ParentBranch: "main", ForkPoint: mainNodes[2].ID.String()},
	}

	chat := &thinkgraph.ChatAddress{
		ChatID:    "test-chat",
		ProjectID: "space",
		SubGraph: thinkgraph.SubGraph{
			Name: "branchA",
		},
	}

	// Requesting 3 items should get the 2 from BranchA and hop back to grab the latest 1 from Main
	page, nextCursor, hasMore, err := engine.GetTimelinePage(ctx, chat, "", 3, topology)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(page) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(page))
	}

	if page[2].Content != "branchA-1" || page[1].Content != "branchA-0" || page[0].Content != "main-2" {
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
	branchNodes := []thinkgraph.Node{{Event: thinkgraph.NewEvent(thinkgraph.EventPrompt, "branch-1")}}

	repo := &mockGraphRepoForTraversal{
		nodesByBranch: map[string][]thinkgraph.Node{
			"main":    mainNodes,
			"branchA": branchNodes,
		},
	}
	engine := thinkgraph.NewTraversalEngine(repo)

	topology := map[string]thinkgraph.BranchTopology{
		"branchA": {ParentBranch: "main", ForkPoint: mainNodes[0].ID.String()},
	}

	chat := &thinkgraph.ChatAddress{
		ChatID:    "test-chat",
		ProjectID: "space",
		SubGraph:  thinkgraph.SubGraph{Name: "branchA"},
	}

	timeline, err := engine.GetFullTimeline(ctx, chat, topology)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(timeline) != 2 {
		t.Fatalf("expected 2 total nodes stitched together, got %d", len(timeline))
	}
	if timeline[0].Content != "main-start" || timeline[1].Content != "branch-1" {
		t.Errorf("Timeline returned in wrong chronological order")
	}
}
