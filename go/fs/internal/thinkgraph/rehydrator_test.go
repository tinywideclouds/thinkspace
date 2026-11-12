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

	mut := thinkgraph.ContentMutation{
		BaseMutation: thinkgraph.BaseMutation{
			TargetID:  targetEv.ID,
			Type:      thinkgraph.MutationContent,
			Timestamp: time.Now(),
		},
		Content: "Updated content!",
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

func TestRehydrator_PrefixRouting(t *testing.T) {
	workspaceRoot := t.TempDir()
	spaceID := "golang"
	chatID := "chat-prefix"

	chatFlowAgent1 := &thinkgraph.ChatAddress{
		ProjectID: spaceID,
		ChatID:    chatID,
		SubGraph: thinkgraph.SubGraph{
			Name: "flow-123/agent-1",
		},
	}
	chatFlowAgent2 := &thinkgraph.ChatAddress{
		ProjectID: spaceID,
		ChatID:    chatID,
		SubGraph: thinkgraph.SubGraph{
			Name: "flow-123/agent-2",
		},
	}

	ledger := thinkgraph.NewFileLedger(1000, workspaceRoot)

	ev1 := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Task for agent 1")
	ev1.Metadata = map[string]string{"agent_id": "agent-1"}
	_ = ledger.AppendEvent(context.Background(), chatFlowAgent1, ev1)

	ev2 := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Task for agent 2")
	ev2.Metadata = map[string]string{"agent_id": "agent-2"}
	_ = ledger.AppendEvent(context.Background(), chatFlowAgent2, ev2)

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graph, _ := db.NewBadgerGraph(opts)
	defer graph.Close()

	graphStore := thinkgraph.NewGraphStore(graph, nil)
	rehydrator := thinkgraph.NewRehydrator(graphStore, workspaceRoot)

	ctx := context.Background()

	// Initial CheckSync for cold database with disk events expects SyncStatusMismatch
	status, diskMan, _, err := rehydrator.CheckSync(ctx, chatFlowAgent1)
	if err != nil {
		t.Fatalf("unexpected CheckSync error: %v", err)
	}
	if status != thinkgraph.SyncStatusMismatch {
		t.Errorf("Expected Mismatch status on cold DB with disk events, got %v", status)
	}
	if diskMan.TotalEvents != 2 {
		t.Errorf("Expected physical disk manifest to reflect 2 total multiplexed events, got %d", diskMan.TotalEvents)
	}

	err = rehydrator.Rebuild(ctx, chatFlowAgent1)
	if err != nil {
		t.Fatalf("unexpected Rebuild error: %v", err)
	}

	t1, _ := graph.GetSubGraphTimeline(ctx, chatFlowAgent1, chatFlowAgent1.GraphID)
	if len(t1) != 1 || t1[0].Content != "Task for agent 1" {
		t.Errorf("Expected exactly 1 logically separated node for agent-1, got %d nodes", len(t1))
	}

	t2, _ := graph.GetSubGraphTimeline(ctx, chatFlowAgent2, chatFlowAgent2.GraphID)
	if len(t2) != 1 || t2[0].Content != "Task for agent 2" {
		t.Errorf("Expected exactly 1 logically separated node for agent-2, got %d nodes", len(t2))
	}
}
