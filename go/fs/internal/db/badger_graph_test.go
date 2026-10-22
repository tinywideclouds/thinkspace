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

	chat := &thinkgraph.Chat{ID: "chat-123", SpaceID: "test-space"}

	event1 := thinkgraph.NewEvent(thinkgraph.EventPrompt, "First event")
	event1.Tags = []string{"init"}
	event1.Timestamp = time.Now().Add(-1 * time.Hour)

	event2 := thinkgraph.NewEvent(thinkgraph.EventModel, "Second event")
	event2.Tags = []string{"response", "init"}
	event2.Timestamp = time.Now()

	if err := graph.AppendEvent(ctx, chat, event1); err != nil {
		t.Fatalf("Failed to append event 1: %v", err)
	}
	if err := graph.AppendEvent(ctx, chat, event2); err != nil {
		t.Fatalf("Failed to append event 2: %v", err)
	}

	timeline, err := graph.GetTimeline(ctx, chat)
	if err != nil {
		t.Fatalf("Failed to get timeline: %v", err)
	}

	if len(timeline) != 2 {
		t.Fatalf("Expected 2 events in timeline, got %d", len(timeline))
	}
}

func TestBadgerGraph_GetTimelinePage(t *testing.T) {
	graph := setupTestGraph(t)
	ctx := context.Background()
	chat := &thinkgraph.Chat{ID: "chat-page", SpaceID: "test-space"}

	var events []thinkgraph.Event
	for i := 0; i < 5; i++ {
		ev := thinkgraph.NewEvent(thinkgraph.EventPrompt, fmt.Sprintf("Event %d", i))
		ev.Timestamp = time.Now().Add(time.Duration(i) * time.Minute)
		_ = graph.AppendEvent(ctx, chat, ev)
		events = append(events, ev)
	}

	// 1. Fetch latest page without cursor
	page1, nextCursor, hasMore, err := graph.GetTimelinePage(ctx, chat, "", 2)
	if err != nil {
		t.Fatalf("Failed to fetch page 1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(page1))
	}
	if !hasMore {
		t.Fatalf("Expected hasMore to be true")
	}
	if page1[1].ID != events[4].ID || page1[0].ID != events[3].ID {
		t.Fatalf("Expected latest 2 events, got %s and %s", page1[0].Content, page1[1].Content)
	}

	// 2. Fetch older page using cursor
	page2, nextCursor2, hasMore2, err := graph.GetTimelinePage(ctx, chat, nextCursor, 2)
	if err != nil {
		t.Fatalf("Failed to fetch page 2: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(page2))
	}
	if !hasMore2 {
		t.Fatalf("Expected hasMore to be true")
	}
	if page2[1].ID != events[2].ID || page2[0].ID != events[1].ID {
		t.Fatalf("Expected older 2 events, got %s and %s", page2[0].Content, page2[1].Content)
	}

	// 3. Fetch final page using nextCursor2
	page3, _, hasMore3, err := graph.GetTimelinePage(ctx, chat, nextCursor2, 2)
	if err != nil {
		t.Fatalf("Failed to fetch page 3: %v", err)
	}
	if len(page3) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(page3))
	}
	if hasMore3 {
		t.Fatalf("Expected hasMore to be false")
	}
	if page3[0].ID != events[0].ID {
		t.Fatalf("Expected oldest event, got %s", page3[0].Content)
	}
}

func TestBadgerGraph_Mutations_TombstoneAndUpdate(t *testing.T) {
	graph := setupTestGraph(t)
	ctx := context.Background()
	chat := &thinkgraph.Chat{ID: "chat-mutations", SpaceID: "test-space"}

	event1 := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Will be deleted")
	event2 := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Will be updated")

	_ = graph.AppendEvent(ctx, chat, event1)
	_ = graph.AppendEvent(ctx, chat, event2)

	t1, _ := graph.GetTimeline(ctx, chat)
	if len(t1) != 2 {
		t.Fatalf("Expected 2 events baseline, got %d", len(t1))
	}

	mut1 := thinkgraph.MutationEvent{
		TargetID:  event1.ID,
		Type:      thinkgraph.MutationTombstone,
		Timestamp: time.Now(),
	}
	if err := graph.SaveMutation(ctx, chat, mut1); err != nil {
		t.Fatalf("Failed to save tombstone mutation: %v", err)
	}

	t2, _ := graph.GetTimeline(ctx, chat)
	if len(t2) != 1 || t2[0].ID != event2.ID {
		t.Fatalf("Tombstoned event was not filtered out of timeline")
	}

	mut2 := thinkgraph.MutationEvent{
		TargetID:  event2.ID,
		Type:      thinkgraph.MutationUpdate,
		Timestamp: time.Now(),
		Payload:   "Updated content!",
	}
	_ = graph.SaveMutation(ctx, chat, mut2)

	t3, _ := graph.GetTimeline(ctx, chat)
	if t3[0].Content != "Updated content!" {
		t.Errorf("Mutation update failed, content is: %s", t3[0].Content)
	}

	mut3 := thinkgraph.MutationEvent{
		TargetID:  event1.ID,
		Type:      thinkgraph.MutationRestore,
		Timestamp: time.Now(),
	}
	_ = graph.SaveMutation(ctx, chat, mut3)

	t4, _ := graph.GetTimeline(ctx, chat)
	if len(t4) != 2 {
		t.Fatalf("Failed to restore tombstoned event")
	}
}

func TestBadgerGraph_GetLenses(t *testing.T) {
	graph := setupTestGraph(t)
	ctx := context.Background()
	chat := &thinkgraph.Chat{ID: "chat-lenses", SpaceID: "test-space"}

	event1 := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Query DB")
	event1.Tags = []string{"database", "backend"}

	digest := thinkgraph.DigestMeta{
		ID:      uuid.NewV7(),
		Summary: "DB Schema decided",
		Tags:    []string{"database"},
	}

	spaceCtx := thinkgraph.SpaceContext{
		ID:      uuid.NewV7(),
		Content: "Always use PostgreSQL",
		Tags:    []string{"database"},
	}

	_ = graph.AppendEvent(ctx, chat, event1)
	_ = graph.SaveDigest(ctx, chat, digest)
	_ = graph.SaveSpaceContext(ctx, chat, spaceCtx)

	result, err := graph.GetLenses(ctx, chat, "database")
	if err != nil {
		t.Fatalf("GetLenses failed: %v", err)
	}

	if len(result.Events) != 1 || result.Events[0].ID != event1.ID {
		t.Errorf("Lens failed to return correct Event")
	}
	if len(result.Digests) != 1 || result.Digests[0].ID != digest.ID {
		t.Errorf("Lens failed to return correct Digest")
	}
	if len(result.SpaceContexts) != 1 || result.SpaceContexts[0].ID != spaceCtx.ID {
		t.Errorf("Lens failed to return correct SpaceContext")
	}
}

func TestBadgerGraph_GetEvents(t *testing.T) {
	graph := setupTestGraph(t)
	ctx := context.Background()
	chat := &thinkgraph.Chat{ID: "chat-events", SpaceID: "test-space"}

	event1 := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Event 1")
	event2 := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Event 2")
	event3 := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Event 3")

	_ = graph.AppendEvent(ctx, chat, event1)
	_ = graph.AppendEvent(ctx, chat, event2)
	_ = graph.AppendEvent(ctx, chat, event3)

	ids := []string{event1.ID.String(), event3.ID.String(), "missing-id-123"}
	events, err := graph.GetEvents(ctx, chat, ids)
	if err != nil {
		t.Fatalf("GetEvents failed: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("Expected exactly 2 events fetched, got %d", len(events))
	}
}

func TestBadgerGraph_Digests(t *testing.T) {
	graph := setupTestGraph(t)
	ctx := context.Background()
	chat := &thinkgraph.Chat{ID: "chat-digests", SpaceID: "test-space"}

	linkedID := uuid.NewV7()
	promotedDigest := thinkgraph.DigestMeta{
		ID:               uuid.NewV7(),
		Summary:          "Architecture rules.",
		Promoted:         true,
		SummarizedEvents: []uuid.UUID{linkedID},
	}

	normalDigest := thinkgraph.DigestMeta{
		ID:       uuid.NewV7(),
		Summary:  "Older discussion about auth.",
		Promoted: false,
	}

	if err := graph.SaveDigest(ctx, chat, promotedDigest); err != nil {
		t.Fatalf("Failed to save sticky digest: %v", err)
	}
	if err := graph.SaveDigest(ctx, chat, normalDigest); err != nil {
		t.Fatalf("Failed to save normal digest: %v", err)
	}

	stickies, err := graph.GetStickyDigests(ctx, chat)
	if err != nil {
		t.Fatalf("GetStickyDigests failed: %v", err)
	}

	if len(stickies) != 1 {
		t.Fatalf("Expected exactly 1 sticky digest, got %d", len(stickies))
	}
	if stickies[0].ID != promotedDigest.ID {
		t.Errorf("Retrieved incorrect sticky digest")
	}

	allDigests, err := graph.GetDigests(ctx, chat)
	if err != nil {
		t.Fatalf("GetDigests failed: %v", err)
	}
	if len(allDigests) != 2 {
		t.Fatalf("Expected exactly 2 total digests, got %d", len(allDigests))
	}
}

func TestBadgerGraph_ClearChatGraph(t *testing.T) {
	graph := setupTestGraph(t)
	ctx := context.Background()
	chat := &thinkgraph.Chat{ID: "chat-clear", SpaceID: "test-space"}

	_ = graph.AppendEvent(ctx, chat, thinkgraph.NewEvent(thinkgraph.EventPrompt, "Test"))

	if err := graph.ClearChatGraph(ctx, chat); err != nil {
		t.Fatalf("ClearChatGraph failed: %v", err)
	}
}

func TestBadgerGraph_ClearChatGraph_PrefixCollision(t *testing.T) {
	graph := setupTestGraph(t)
	ctx := context.Background()

	chat1 := &thinkgraph.Chat{ID: "chat-1", SpaceID: "test-space"}
	chat11 := &thinkgraph.Chat{ID: "chat-11", SpaceID: "test-space"}

	_ = graph.AppendEvent(ctx, chat1, thinkgraph.NewEvent(thinkgraph.EventPrompt, "Event for chat 1"))
	_ = graph.AppendEvent(ctx, chat11, thinkgraph.NewEvent(thinkgraph.EventPrompt, "Event for chat 11"))

	if err := graph.ClearChatGraph(ctx, chat1); err != nil {
		t.Fatalf("ClearChatGraph failed: %v", err)
	}

	t1, _ := graph.GetTimeline(ctx, chat1)
	if len(t1) != 0 {
		t.Errorf("Expected chat-1 to be wiped, got %d events remaining", len(t1))
	}

	t11, _ := graph.GetTimeline(ctx, chat11)
	if len(t11) != 1 {
		t.Errorf("Expected chat-11 to be untouched, got %d events. Prefix collision occurred!", len(t11))
	}
}
