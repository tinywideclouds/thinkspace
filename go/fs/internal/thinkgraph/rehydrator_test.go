package thinkgraph_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/tinywideclouds.com/thinkspace/internal/db"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

func TestRehydrator_SyncScenarios(t *testing.T) {
	tempDir := t.TempDir()
	workspaceRoot := tempDir
	spaceID := "test-space"
	chatID := "chat-rehydrate"

	chat := &thinkgraph.Chat{
		ID:      chatID,
		SpaceID: spaceID,
	}

	// 1. Scaffold physical disk
	ledgerPath := filepath.Join(workspaceRoot, spaceID, "chats", chatID, "ledger")
	logsDir := filepath.Join(ledgerPath, "logs")
	digestsDir := filepath.Join(ledgerPath, "digests")
	os.MkdirAll(logsDir, 0755)
	os.MkdirAll(digestsDir, 0755)

	ledger := thinkgraph.NewFileLedger(1000, workspaceRoot)
	var originalEvents []thinkgraph.Event
	for i := 0; i < 5; i++ {
		event := thinkgraph.NewEvent(thinkgraph.EventPrompt, fmt.Sprintf("Test prompt %d", i))
		event.Timestamp = time.Now().UTC()
		_ = ledger.AppendEvent(context.Background(), chat, event)
		originalEvents = append(originalEvents, event)
		time.Sleep(1 * time.Millisecond) // Ensure deterministic temporal ordering
	}

	// 2. Scaffold Badger
	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graph, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("Failed to initialize badger: %v", err)
	}
	defer graph.Close()

	rehydrator := thinkgraph.NewRehydrator(graph, workspaceRoot)

	// Scenario A: Cold Boot / Hard Mismatch
	status, diskMan, graphMan, err := rehydrator.CheckSync(context.Background(), chat)
	if err != nil {
		t.Fatalf("CheckSync failed: %v", err)
	}
	if status != thinkgraph.SyncStatusMismatch {
		t.Fatalf("Expected Mismatch on cold boot, got %s", status)
	}
	if diskMan.TotalEvents != 5 {
		t.Errorf("Expected disk manifest to reflect 5 events, got %d", diskMan.TotalEvents)
	}
	if graphMan != nil {
		t.Errorf("Expected nil graph manifest on cold boot")
	}

	// Scenario B: Explicit Rebuild
	err = rehydrator.Rebuild(context.Background(), chat)
	if err != nil {
		t.Fatalf("Rebuild failed: %v", err)
	}

	status, _, graphMan, _ = rehydrator.CheckSync(context.Background(), chat)
	if status != thinkgraph.SyncStatusOK {
		t.Fatalf("Expected OK after rebuild, got %s", status)
	}
	if graphMan.TotalEvents != 5 {
		t.Errorf("Expected badger to hold 5 events, got %d", graphMan.TotalEvents)
	}

	// Verify timeline integrity after rebuild
	timeline, _ := graph.GetTimeline(context.Background(), chat)
	if len(timeline) != 5 {
		t.Fatalf("Expected timeline to have 5 events, got %d", len(timeline))
	}
	for i, ev := range timeline {
		if ev.ID != originalEvents[i].ID {
			t.Errorf("Rebuild corrupted event ordering at index %d", i)
		}
	}

	// Scenario C: Graph is behind (Fast-Forward scenario)
	// Simulate writing multiple events directly to disk to expose the skip bug
	var newEvents []thinkgraph.Event
	for i := 0; i < 4; i++ {
		newEvent := thinkgraph.NewEvent(thinkgraph.EventPrompt, fmt.Sprintf("New Event %d", i))
		newEvent.Timestamp = time.Now().UTC()
		_ = ledger.AppendEvent(context.Background(), chat, newEvent)
		newEvents = append(newEvents, newEvent)
		time.Sleep(1 * time.Millisecond)
	}

	status, diskMan, graphMan, _ = rehydrator.CheckSync(context.Background(), chat)
	if status != thinkgraph.SyncStatusBehind {
		t.Fatalf("Expected Behind status after disk advance, got %s", status)
	}

	// Execute Fast-Forward
	err = rehydrator.FastForward(context.Background(), chat, graphMan, diskMan)
	if err != nil {
		t.Fatalf("FastForward failed: %v", err)
	}

	status, _, graphMan, _ = rehydrator.CheckSync(context.Background(), chat)
	if status != thinkgraph.SyncStatusOK {
		t.Fatalf("Expected OK after fast-forward, got %s", status)
	}
	if graphMan.TotalEvents != 9 {
		t.Errorf("Expected badger to hold 9 events after fast-forward, got %d", graphMan.TotalEvents)
	}

	// Verify timeline integrity after fast forward
	ffTimeline, _ := graph.GetTimeline(context.Background(), chat)
	if len(ffTimeline) != 9 {
		t.Fatalf("Expected timeline to have 9 events, got %d", len(ffTimeline))
	}

	// Ensure the 4 new events were appended in the exact correct order without skipping
	for i := 0; i < 4; i++ {
		if ffTimeline[5+i].ID != newEvents[i].ID {
			t.Errorf("FastForward skipped or corrupted events. Expected ID %s at index %d, got %s", newEvents[i].ID, 5+i, ffTimeline[5+i].ID)
		}
	}
}
