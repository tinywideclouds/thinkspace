package db_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"uuid"

	"github.com/dgraph-io/badger/v4"
	"github.com/tinywideclouds.com/thinkspace/internal/db"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

func setupTestGraph(t *testing.T) *db.BadgerGraph {
	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graph, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("Failed to initialize in-memory badger graph: %v", err)
	}
	t.Cleanup(func() {
		graph.Close()
	})
	return graph
}

func TestBadgerGraph_AppendAndGetTimeline(t *testing.T) {
	graph := setupTestGraph(t)
	ctx := context.Background()

	chat := &thinkgraph.Chat{ID: "chat-123", SpaceID: "test-space", ActiveBranch: "main"}

	node1 := thinkgraph.Node{Event: thinkgraph.NewEvent(thinkgraph.EventPrompt, "First event")}
	node1.Tags = []string{"init"}
	node1.Timestamp = time.Now().Add(-1 * time.Hour)
	node1.BranchID = "main"

	node2 := thinkgraph.Node{Event: thinkgraph.NewEvent(thinkgraph.EventModel, "Second event")}
	node2.Tags = []string{"response", "init"}
	node2.Timestamp = time.Now()
	node2.BranchID = "main"

	if err := graph.AppendNode(ctx, chat, node1); err != nil {
		t.Fatalf("Failed to append node 1: %v", err)
	}
	if err := graph.AppendNode(ctx, chat, node2); err != nil {
		t.Fatalf("Failed to append node 2: %v", err)
	}

	timeline, err := graph.GetBranchTimeline(ctx, chat, "main")
	if err != nil {
		t.Fatalf("Failed to get timeline: %v", err)
	}

	if len(timeline) != 2 {
		t.Fatalf("Expected 2 nodes in timeline, got %d", len(timeline))
	}
}

func TestBadgerGraph_GetTimelinePage(t *testing.T) {
	graph := setupTestGraph(t)
	ctx := context.Background()
	chat := &thinkgraph.Chat{ID: "chat-page", SpaceID: "test-space", ActiveBranch: "main"}

	var nodes []thinkgraph.Node
	for i := 0; i < 5; i++ {
		ev := thinkgraph.NewEvent(thinkgraph.EventPrompt, fmt.Sprintf("Event %d", i))
		ev.Timestamp = time.Now().Add(time.Duration(i) * time.Minute)
		ev.BranchID = "main"
		node := thinkgraph.Node{Event: ev}
		_ = graph.AppendNode(ctx, chat, node)
		nodes = append(nodes, node)
	}

	page1, nextCursor, hasMore, err := graph.GetBranchTimelinePage(ctx, chat, "main", "", 2)
	if err != nil {
		t.Fatalf("Failed to fetch page 1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("Expected 2 nodes, got %d", len(page1))
	}
	if !hasMore {
		t.Fatalf("Expected hasMore to be true")
	}
	if page1[1].ID != nodes[4].ID || page1[0].ID != nodes[3].ID {
		t.Fatalf("Expected latest 2 nodes, got %s and %s", page1[0].Content, page1[1].Content)
	}

	page2, nextCursor2, hasMore2, err := graph.GetBranchTimelinePage(ctx, chat, "main", nextCursor, 2)
	if err != nil {
		t.Fatalf("Failed to fetch page 2: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("Expected 2 nodes, got %d", len(page2))
	}
	if !hasMore2 {
		t.Fatalf("Expected hasMore to be true")
	}
	if page2[1].ID != nodes[2].ID || page2[0].ID != nodes[1].ID {
		t.Fatalf("Expected older 2 nodes, got %s and %s", page2[0].Content, page2[1].Content)
	}

	page3, _, hasMore3, err := graph.GetBranchTimelinePage(ctx, chat, "main", nextCursor2, 2)
	if err != nil {
		t.Fatalf("Failed to fetch page 3: %v", err)
	}
	if len(page3) != 1 {
		t.Fatalf("Expected 1 node, got %d", len(page3))
	}
	if hasMore3 {
		t.Fatalf("Expected hasMore to be false")
	}
	if page3[0].ID != nodes[0].ID {
		t.Fatalf("Expected oldest node, got %s", page3[0].Content)
	}
}

func TestBadgerGraph_Mutations_TombstoneAndUpdate(t *testing.T) {
	graph := setupTestGraph(t)
	ctx := context.Background()
	chat := &thinkgraph.Chat{ID: "chat-mutations", SpaceID: "test-space", ActiveBranch: "main"}

	ev1 := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Will be deleted")
	ev1.BranchID = "main"
	node1 := thinkgraph.Node{Event: ev1}

	ev2 := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Will be updated")
	ev2.BranchID = "main"
	node2 := thinkgraph.Node{Event: ev2}

	_ = graph.AppendNode(ctx, chat, node1)
	_ = graph.AppendNode(ctx, chat, node2)

	t1, _ := graph.GetBranchTimeline(ctx, chat, "main")
	if len(t1) != 2 {
		t.Fatalf("Expected 2 nodes baseline, got %d", len(t1))
	}

	mut1 := thinkgraph.MutationEvent{
		TargetID:  node1.ID,
		Type:      thinkgraph.MutationTombstone,
		Timestamp: time.Now(),
	}
	if err := graph.SaveMutation(ctx, chat, mut1); err != nil {
		t.Fatalf("Failed to save tombstone mutation: %v", err)
	}

	t2, _ := graph.GetBranchTimeline(ctx, chat, "main")
	if len(t2) != 1 || t2[0].ID != node2.ID {
		t.Fatalf("Tombstoned node was not filtered out of timeline")
	}

	mut2 := thinkgraph.MutationEvent{
		TargetID:  node2.ID,
		Type:      thinkgraph.MutationUpdate,
		Timestamp: time.Now(),
		Payload:   "Updated content!",
	}
	_ = graph.SaveMutation(ctx, chat, mut2)

	t3, _ := graph.GetBranchTimeline(ctx, chat, "main")
	if t3[0].Content != "Updated content!" {
		t.Errorf("Mutation update failed, content is: %s", t3[0].Content)
	}

	mut3 := thinkgraph.MutationEvent{
		TargetID:  node1.ID,
		Type:      thinkgraph.MutationRestore,
		Timestamp: time.Now(),
	}
	_ = graph.SaveMutation(ctx, chat, mut3)

	t4, _ := graph.GetBranchTimeline(ctx, chat, "main")
	if len(t4) != 2 {
		t.Fatalf("Failed to restore tombstoned node")
	}
}

func TestBadgerGraph_GetLenses(t *testing.T) {
	graph := setupTestGraph(t)
	ctx := context.Background()
	chat := &thinkgraph.Chat{ID: "chat-lenses", SpaceID: "test-space", ActiveBranch: "main"}

	ev1 := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Query DB")
	ev1.Tags = []string{"database", "backend"}
	ev1.BranchID = "main"
	node1 := thinkgraph.Node{Event: ev1}

	digestID := uuid.NewV7()
	digest := thinkgraph.DigestMeta{
		ID:      digestID,
		Summary: "DB Schema decided",
		Tags:    []string{"database"},
	}

	spaceCtxID := uuid.NewV7()
	spaceCtx := thinkgraph.SpaceContext{
		ID:      spaceCtxID,
		Content: "Always use PostgreSQL",
		Tags:    []string{"database"},
	}

	_ = graph.AppendNode(ctx, chat, node1)
	_ = graph.SaveDigest(ctx, chat, digest)
	_ = graph.SaveSpaceContext(ctx, chat, spaceCtx)

	result, err := graph.GetLenses(ctx, chat, "database")
	if err != nil {
		t.Fatalf("GetLenses failed: %v", err)
	}

	if len(result.Nodes) != 1 || result.Nodes[0].ID != node1.ID {
		t.Errorf("Lens failed to return correct Node")
	}
	if len(result.Digests) != 1 || result.Digests[0].ID != digest.ID {
		t.Errorf("Lens failed to return correct Digest")
	}
	if len(result.SpaceContexts) != 1 || result.SpaceContexts[0].ID != spaceCtx.ID {
		t.Errorf("Lens failed to return correct SpaceContext")
	}
}
