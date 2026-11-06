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
	subGraphName := "main"
	graphID := thinkgraph.NewGraphID(thinkgraph.PrefixChat)

	chat := &thinkgraph.ChatAddress{
		ChatID:    chatID,
		ProjectID: spaceID,
		SubGraph: thinkgraph.SubGraph{
			Name:    subGraphName,
			GraphID: graphID,
		},
	}

	ledgerPath := filepath.Join(workspaceRoot, spaceID, "chats", chatID, "ledger")
	logsDir := filepath.Join(ledgerPath, "logs", subGraphName)
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
		time.Sleep(1 * time.Millisecond)
	}

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graph, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("Failed to initialize badger: %v", err)
	}
	defer graph.Close()

	graphStore := thinkgraph.NewGraphStore(graph, nil)
	rehydrator := thinkgraph.NewRehydrator(graphStore, workspaceRoot)

	status, diskManifest, graphManifest, err := rehydrator.CheckSync(context.Background(), chat)
	if err != nil {
		t.Fatalf("CheckSync failed: %v", err)
	}
	if status != thinkgraph.SyncStatusMismatch {
		t.Fatalf("Expected Mismatch on cold boot, got %s", status)
	}
	if diskManifest.TotalEvents != 5 {
		t.Errorf("Expected disk manifest to reflect 5 events, got %d", diskManifest.TotalEvents)
	}
	if graphManifest != nil {
		t.Errorf("Expected nil graph manifest on cold boot")
	}

	err = rehydrator.Rebuild(context.Background(), chat)
	if err != nil {
		t.Fatalf("Rebuild failed: %v", err)
	}

	status, _, graphManifest, _ = rehydrator.CheckSync(context.Background(), chat)
	if status != thinkgraph.SyncStatusOK {
		t.Fatalf("Expected OK after rebuild, got %s", status)
	}
	if graphManifest.TotalEvents != 5 {
		t.Errorf("Expected badger to hold 5 events, got %d", graphManifest.TotalEvents)
	}

	timeline, _ := graph.GetSubGraphTimeline(context.Background(), chat, graphID)
	if len(timeline) != 5 {
		t.Fatalf("Expected timeline to have 5 nodes, got %d", len(timeline))
	}
	for i, node := range timeline {
		if node.ID != originalEvents[i].ID {
			t.Errorf("Rebuild corrupted node ordering at index %d", i)
		}
	}

	var newEvents []thinkgraph.Event
	for i := 0; i < 4; i++ {
		newEvent := thinkgraph.NewEvent(thinkgraph.EventPrompt, fmt.Sprintf("New Event %d", i))
		newEvent.Timestamp = time.Now().UTC()
		_ = ledger.AppendEvent(context.Background(), chat, newEvent)
		newEvents = append(newEvents, newEvent)
		time.Sleep(1 * time.Millisecond)
	}

	status, diskManifest, graphManifest, _ = rehydrator.CheckSync(context.Background(), chat)
	if status != thinkgraph.SyncStatusBehind {
		t.Fatalf("Expected Behind status after disk advance, got %s", status)
	}

	err = rehydrator.FastForward(context.Background(), chat, graphManifest, diskManifest)
	if err != nil {
		t.Fatalf("FastForward failed: %v", err)
	}

	ffTimeline, _ := graph.GetSubGraphTimeline(context.Background(), chat, graphID)
	if len(ffTimeline) != 9 {
		t.Fatalf("Expected timeline to have 9 nodes, got %d", len(ffTimeline))
	}
}

func TestRehydrator_MutationReplay(t *testing.T) {
	tempDir := t.TempDir()
	graphID := thinkgraph.NewGraphID(thinkgraph.PrefixChat)
	chat := &thinkgraph.ChatAddress{
		ChatID:    "chat-mut",
		ProjectID: "space",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: graphID,
		},
	}

	ledger := thinkgraph.NewFileLedger(1000, tempDir)

	targetEv := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Should be updated")
	_ = ledger.AppendEvent(context.Background(), chat, targetEv)

	mut := thinkgraph.MutationEvent{
		TargetID:  targetEv.ID,
		Type:      thinkgraph.MutationUpdate,
		Timestamp: time.Now(),
		Payload:   "Updated content!",
	}
	_ = ledger.SaveMutation(context.Background(), chat, mut)

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graph, _ := db.NewBadgerGraph(opts)
	defer graph.Close()

	graphStore := thinkgraph.NewGraphStore(graph, nil)
	rehydrator := thinkgraph.NewRehydrator(graphStore, tempDir)
	_ = rehydrator.Rebuild(context.Background(), chat)

	t1, _ := graph.GetSubGraphTimeline(context.Background(), chat, graphID)
	if len(t1) != 1 {
		t.Fatalf("Expected 1 node, got %d", len(t1))
	}

	if t1[0].Content != "Updated content!" {
		t.Errorf("Mutation was not replayed correctly during rebuild, got: %s", t1[0].Content)
	}
}
