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
	t.Helper()
	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graph, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("Failed to initialize in-memory badger graph: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := graph.Close(); closeErr != nil {
			t.Errorf("failed closing badger graph: %v", closeErr)
		}
	})
	return graph
}

func TestBadgerGraph_AppendAndGetTimeline(t *testing.T) {
	graph := setupTestGraph(t)
	ctx := context.Background()

	graphID := thinkgraph.NewGraphID(thinkgraph.PrefixChat)
	chat := &thinkgraph.ChatAddress{
		ChatID:    "chat-123",
		ProjectID: "test-space",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: graphID,
		},
	}

	node1 := thinkgraph.Node{Event: thinkgraph.NewEvent(thinkgraph.EventPrompt, "First event")}
	node1.Tags = []string{"init"}
	node1.Timestamp = time.Now().Add(-1 * time.Hour)
	node1.GraphID = graphID

	node2 := thinkgraph.Node{Event: thinkgraph.NewEvent(thinkgraph.EventModel, "Second event")}
	node2.Tags = []string{"response", "init"}
	node2.Timestamp = time.Now()
	node2.GraphID = graphID

	if err := graph.AppendNode(ctx, chat, node1); err != nil {
		t.Fatalf("Failed to append node 1: %v", err)
	}
	if err := graph.AppendNode(ctx, chat, node2); err != nil {
		t.Fatalf("Failed to append node 2: %v", err)
	}

	timeline, err := graph.GetSubGraphTimeline(ctx, chat, graphID)
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
	graphID := thinkgraph.NewGraphID(thinkgraph.PrefixChat)
	chat := &thinkgraph.ChatAddress{
		ChatID:    "chat-page",
		ProjectID: "test-space",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: graphID,
		},
	}

	var nodes []thinkgraph.Node
	for i := 0; i < 5; i++ {
		ev := thinkgraph.NewEvent(thinkgraph.EventPrompt, fmt.Sprintf("Event %d", i))
		ev.Timestamp = time.Now().Add(time.Duration(i) * time.Minute)
		ev.GraphID = graphID
		node := thinkgraph.Node{Event: ev}
		if err := graph.AppendNode(ctx, chat, node); err != nil {
			t.Fatalf("AppendNode %d failed: %v", i, err)
		}
		nodes = append(nodes, node)
	}

	page1, nextCursor, hasMore, err := graph.GetSubGraphTimelinePage(ctx, chat, graphID, "", 2)
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

	page2, nextCursor2, hasMore2, err := graph.GetSubGraphTimelinePage(ctx, chat, graphID, nextCursor, 2)
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

	page3, _, hasMore3, err := graph.GetSubGraphTimelinePage(ctx, chat, graphID, nextCursor2, 2)
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
	graphID := thinkgraph.NewGraphID(thinkgraph.PrefixChat)
	chat := &thinkgraph.ChatAddress{
		ChatID:    "chat-mutations",
		ProjectID: "test-space",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: graphID,
		},
	}

	ev1 := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Will be deleted")
	ev1.GraphID = graphID
	node1 := thinkgraph.Node{Event: ev1}

	ev2 := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Will be updated")
	ev2.GraphID = graphID
	node2 := thinkgraph.Node{Event: ev2}

	if err := graph.AppendNode(ctx, chat, node1); err != nil {
		t.Fatalf("AppendNode 1 failed: %v", err)
	}
	if err := graph.AppendNode(ctx, chat, node2); err != nil {
		t.Fatalf("AppendNode 2 failed: %v", err)
	}

	t1, err := graph.GetSubGraphTimeline(ctx, chat, graphID)
	if err != nil || len(t1) != 2 {
		t.Fatalf("Expected 2 nodes baseline, got %d (err=%v)", len(t1), err)
	}

	mut1 := thinkgraph.TombstoneMutation{
		BaseMutation: thinkgraph.BaseMutation{
			TargetID:  node1.ID,
			Type:      thinkgraph.MutationTombstone,
			Timestamp: time.Now(),
		},
	}
	if err := graph.SaveMutation(ctx, chat, mut1); err != nil {
		t.Fatalf("Failed to save tombstone mutation: %v", err)
	}

	t2, err := graph.GetSubGraphTimeline(ctx, chat, graphID)
	if err != nil || len(t2) != 1 || t2[0].ID != node2.ID {
		t.Fatalf("Tombstoned node was not filtered out of timeline (err=%v)", err)
	}

	mut2 := thinkgraph.ContentMutation{
		BaseMutation: thinkgraph.BaseMutation{
			TargetID:  node2.ID,
			Type:      thinkgraph.MutationContent,
			Timestamp: time.Now(),
		},
		Content: "Updated content!",
	}
	if err := graph.SaveMutation(ctx, chat, mut2); err != nil {
		t.Fatalf("SaveMutation ContentMutation failed: %v", err)
	}

	t3, err := graph.GetSubGraphTimeline(ctx, chat, graphID)
	if err != nil || t3[0].Content != "Updated content!" {
		t.Errorf("Mutation update failed, content is: %s (err=%v)", t3[0].Content, err)
	}

	mut3 := thinkgraph.RestoreMutation{
		BaseMutation: thinkgraph.BaseMutation{
			TargetID:  node1.ID,
			Type:      thinkgraph.MutationRestore,
			Timestamp: time.Now(),
		},
	}
	if err := graph.SaveMutation(ctx, chat, mut3); err != nil {
		t.Fatalf("SaveMutation RestoreMutation failed: %v", err)
	}

	t4, err := graph.GetSubGraphTimeline(ctx, chat, graphID)
	if err != nil || len(t4) != 2 {
		t.Fatalf("Failed to restore tombstoned node (err=%v)", err)
	}
}

func TestBadgerGraph_Mutations_StateAndCombinedContentUpdate(t *testing.T) {
	graph := setupTestGraph(t)
	ctx := context.Background()
	graphID := thinkgraph.NewGraphID(thinkgraph.PrefixChat)
	chat := &thinkgraph.ChatAddress{
		ChatID:    "chat-state-mutations",
		ProjectID: "test-space",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: graphID,
		},
	}

	ev1 := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Flow Node")
	ev1.GraphID = graphID
	node1 := thinkgraph.Node{Event: ev1}
	if err := graph.AppendNode(ctx, chat, node1); err != nil {
		t.Fatalf("AppendNode failed: %v", err)
	}

	stateBytes := []byte(`{"status":"running"}`)
	mut := thinkgraph.StateMutation{
		BaseMutation: thinkgraph.BaseMutation{
			TargetID:  node1.ID,
			Type:      thinkgraph.MutationState,
			Timestamp: time.Now(),
		},
		State: stateBytes,
	}

	if err := graph.SaveMutation(ctx, chat, mut); err != nil {
		t.Fatalf("SaveMutation failed: %v", err)
	}

	t1, err := graph.GetSubGraphTimeline(ctx, chat, graphID)
	if err != nil {
		t.Fatalf("GetSubGraphTimeline failed: %v", err)
	}
	if string(t1[0].MaterializedState) != string(stateBytes) {
		t.Errorf("State mutation update failed, got %s", string(t1[0].MaterializedState))
	}
	if t1[0].Content != "Flow Node" {
		t.Errorf("Expected Content to remain 'Flow Node' when StateMutation.Content is empty, got %q", t1[0].Content)
	}

	// Combined StateMutation with Content updates both atomically
	completedState := []byte(`{"status":"completed"}`)
	combinedMut := thinkgraph.StateMutation{
		BaseMutation: thinkgraph.BaseMutation{
			TargetID:  node1.ID,
			Type:      thinkgraph.MutationState,
			Timestamp: time.Now(),
		},
		Content: "Flow Completed Summary",
		State:   completedState,
	}
	if err := graph.SaveMutation(ctx, chat, combinedMut); err != nil {
		t.Fatalf("SaveMutation combined failed: %v", err)
	}

	t2, err := graph.GetSubGraphTimeline(ctx, chat, graphID)
	if err != nil {
		t.Fatalf("GetSubGraphTimeline failed: %v", err)
	}
	if t2[0].Content != "Flow Completed Summary" || string(t2[0].MaterializedState) != string(completedState) {
		t.Errorf("Expected combined state+content update, got content=%q state=%s", t2[0].Content, string(t2[0].MaterializedState))
	}
}

func TestBadgerGraph_SyncStatePerSubGraphAndClear(t *testing.T) {
	graph := setupTestGraph(t)
	ctx := context.Background()

	mainID := thinkgraph.NewGraphID(thinkgraph.PrefixChat)
	mainAddr := &thinkgraph.ChatAddress{
		ChatID:    "chat-sync-isolation",
		ProjectID: "test-space",
		SubGraph:  thinkgraph.SubGraph{Name: "main", GraphID: mainID},
	}

	flowID := thinkgraph.NewGraphID(thinkgraph.PrefixFlow)
	flowAddr := &thinkgraph.ChatAddress{
		ChatID:    "chat-sync-isolation",
		ProjectID: "test-space",
		SubGraph:  thinkgraph.SubGraph{Name: "flow-" + flowID.UUID.String(), GraphID: flowID},
	}

	// Append 2 events to main and 1 event to flow subgraph; their sync states must remain isolated
	for i := 0; i < 2; i++ {
		ev := thinkgraph.NewEvent(thinkgraph.EventPrompt, fmt.Sprintf("main-%d", i))
		ev.GraphID = mainID
		if err := graph.AppendNode(ctx, mainAddr, thinkgraph.Node{Event: ev}); err != nil {
			t.Fatalf("AppendNode(main) failed: %v", err)
		}
	}

	flowEv := thinkgraph.NewEvent(thinkgraph.EventPrompt, "flow-0")
	flowEv.GraphID = flowID
	if err := graph.AppendNode(ctx, flowAddr, thinkgraph.Node{Event: flowEv}); err != nil {
		t.Fatalf("AppendNode(flow) failed: %v", err)
	}

	// Re-appending an existing node ID must be idempotent for TotalEvents
	if err := graph.AppendNode(ctx, flowAddr, thinkgraph.Node{Event: flowEv}); err != nil {
		t.Fatalf("idempotent AppendNode(flow) failed: %v", err)
	}

	mainSync, err := graph.GetSyncState(ctx, mainAddr)
	if err != nil || mainSync == nil || mainSync.TotalEvents != 2 {
		t.Fatalf("expected main sync TotalEvents=2, got %+v (err=%v)", mainSync, err)
	}

	flowSync, err := graph.GetSyncState(ctx, flowAddr)
	if err != nil || flowSync == nil || flowSync.TotalEvents != 1 {
		t.Fatalf("expected flow sync TotalEvents=1, got %+v (err=%v)", flowSync, err)
	}

	// ClearChatGraph must clear sync state across all branches/subgraphs
	if err := graph.ClearChatGraph(ctx, mainAddr); err != nil {
		t.Fatalf("ClearChatGraph failed: %v", err)
	}
	if clearedMain, _ := graph.GetSyncState(ctx, mainAddr); clearedMain != nil {
		t.Errorf("expected main sync state to be cleared, got %+v", clearedMain)
	}
	if clearedFlow, _ := graph.GetSyncState(ctx, flowAddr); clearedFlow != nil {
		t.Errorf("expected flow sync state to be cleared, got %+v", clearedFlow)
	}
}

func TestBadgerGraph_GetLenses(t *testing.T) {
	graph := setupTestGraph(t)
	ctx := context.Background()
	graphID := thinkgraph.NewGraphID(thinkgraph.PrefixChat)
	chat := &thinkgraph.ChatAddress{
		ChatID:    "chat-lenses",
		ProjectID: "test-space",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: graphID,
		},
	}

	ev1 := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Query DB")
	ev1.Tags = []string{"database", "backend"}
	ev1.GraphID = graphID
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

	if err := graph.AppendNode(ctx, chat, node1); err != nil {
		t.Fatalf("AppendNode failed: %v", err)
	}
	if err := graph.SaveDigest(ctx, chat, digest); err != nil {
		t.Fatalf("SaveDigest failed: %v", err)
	}
	if err := graph.SaveSpaceContext(ctx, chat, spaceCtx); err != nil {
		t.Fatalf("SaveSpaceContext failed: %v", err)
	}

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
