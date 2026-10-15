package thinkgraph_test

import (
	"context"
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
	for i := 0; i < 5; i++ {
		event := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Test prompt")
		event.Timestamp = time.Now().UTC()
		_ = ledger.AppendEvent(context.Background(), chat, event)
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

	// Scenario C: Graph is behind (Fast-Forward scenario)
	// Simulate writing directly to disk (e.g., server crashed before graph flush)
	newEvent := thinkgraph.NewEvent(thinkgraph.EventPrompt, "New Event")
	newEvent.Timestamp = time.Now().UTC()
	_ = ledger.AppendEvent(context.Background(), chat, newEvent)

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
	if graphMan.TotalEvents != 6 {
		t.Errorf("Expected badger to hold 6 events after fast-forward, got %d", graphMan.TotalEvents)
	}
}
