package db_test

import (
	"context"
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
	event1.Timestamp = time.Now().Add(-1 * time.Hour) // Ensure explicit chronological order

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

	if timeline[0].ID != event1.ID {
		t.Errorf("Expected first event to be %s, got %s", event1.ID, timeline[0].ID)
	}
	if timeline[1].ID != event2.ID {
		t.Errorf("Expected second event to be %s, got %s", event2.ID, timeline[1].ID)
	}
}

func TestBadgerGraph_GetLenses(t *testing.T) {
	graph := setupTestGraph(t)
	ctx := context.Background()
	chat := &thinkgraph.Chat{ID: "chat-lenses", SpaceID: "test-space"}

	event1 := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Query DB")
	event1.Tags = []string{"database", "backend"}

	event2 := thinkgraph.NewEvent(thinkgraph.EventModel, "DB result")
	event2.Tags = []string{"database"}

	event3 := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Fix UI")
	event3.Tags = []string{"frontend"}

	graph.AppendEvent(ctx, chat, event1)
	graph.AppendEvent(ctx, chat, event2)
	graph.AppendEvent(ctx, chat, event3)

	dbEvents, err := graph.GetLenses(ctx, chat, "database")
	if err != nil {
		t.Fatalf("GetLenses failed: %v", err)
	}

	if len(dbEvents) != 2 {
		t.Fatalf("Expected 2 database events, got %d", len(dbEvents))
	}

	foundE1, foundE2 := false, false
	for _, ev := range dbEvents {
		if ev.ID == event1.ID {
			foundE1 = true
		}
		if ev.ID == event2.ID {
			foundE2 = true
		}
	}
	if !foundE1 || !foundE2 {
		t.Errorf("Lenses did not fetch the correct subset of events")
	}

	frontEvents, _ := graph.GetLenses(ctx, chat, "frontend")
	if len(frontEvents) != 1 || frontEvents[0].ID != event3.ID {
		t.Errorf("Frontend lens failed to fetch event 3 accurately")
	}
}

func TestBadgerGraph_Digests(t *testing.T) {
	graph := setupTestGraph(t)
	ctx := context.Background()
	chat := &thinkgraph.Chat{ID: "chat-digests", SpaceID: "test-space"}

	stickyDigest := thinkgraph.DigestMeta{
		ID:       uuid.NewV7(),
		Summary:  "Architecture rules.",
		IsSticky: true,
	}

	normalDigest := thinkgraph.DigestMeta{
		ID:       uuid.NewV7(),
		Summary:  "Older discussion about auth.",
		IsSticky: false,
	}

	graph.SaveDigest(ctx, chat, stickyDigest)
	graph.SaveDigest(ctx, chat, normalDigest)

	stickies, err := graph.GetStickyDigests(ctx, chat)
	if err != nil {
		t.Fatalf("GetStickyDigests failed: %v", err)
	}

	if len(stickies) != 1 {
		t.Fatalf("Expected exactly 1 sticky digest, got %d", len(stickies))
	}

	if stickies[0].ID != stickyDigest.ID {
		t.Errorf("Retrieved incorrect sticky digest")
	}
}

func TestBadgerGraph_ClearChatGraph(t *testing.T) {
	graph := setupTestGraph(t)
	ctx := context.Background()
	chat := &thinkgraph.Chat{ID: "chat-clear", SpaceID: "test-space"}

	graph.AppendEvent(ctx, chat, thinkgraph.NewEvent(thinkgraph.EventPrompt, "Test"))

	if err := graph.ClearChatGraph(ctx, chat); err != nil {
		t.Fatalf("ClearChatGraph failed: %v", err)
	}
}
