package thinkgraph_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"uuid"

	"github.com/dgraph-io/badger/v4"

	"github.com/tinywideclouds.com/thinkspace/internal/db"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

func TestResolveLogDirName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{input: "", expected: "main"},
		{input: "main", expected: "main"},
		{input: "flow-123/agent-1", expected: "flow-123"},
		{input: "feature/auth", expected: "feature_auth"},
	}

	for _, tt := range tests {
		if got := thinkgraph.ResolveLogDirName(tt.input); got != tt.expected {
			t.Errorf("ResolveLogDirName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestFileLedger_ShardRolloverAndLargePayloadRecovery(t *testing.T) {
	ctx := context.Background()
	workspaceRoot := t.TempDir()
	ledger := thinkgraph.NewFileLedger(2, workspaceRoot)

	chat := &thinkgraph.ChatAddress{
		ProjectID: "space",
		ChatID:    "chat-shards",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}

	// Write 3 events with MaxEventsPerShard=2, including a >128KB payload to exercise large line scanning
	largeContent := strings.Repeat("x", 128*1024)
	ev1 := thinkgraph.NewEvent(thinkgraph.EventPrompt, "event-1")
	ev2 := thinkgraph.NewEvent(thinkgraph.EventModel, largeContent)
	ev3 := thinkgraph.NewEvent(thinkgraph.EventPrompt, "event-3")

	for _, ev := range []*thinkgraph.Event{&ev1, &ev2, &ev3} {
		if err := ledger.AppendEvent(ctx, chat, *ev); err != nil {
			t.Fatalf("AppendEvent failed: %v", err)
		}
	}

	logsDir := filepath.Join(workspaceRoot, chat.ProjectID, "chats", chat.ChatID, "ledger", "logs", "main")
	if _, err := os.Stat(filepath.Join(logsDir, "00000.jsonl")); err != nil {
		t.Errorf("expected shard 00000.jsonl to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(logsDir, "00001.jsonl")); err != nil {
		t.Errorf("expected rolled-over shard 00001.jsonl to exist: %v", err)
	}

	// Delete manifest.json and verify RebuildManifest handles >64KB lines without truncation
	manifestPath := filepath.Join(logsDir, "manifest.json")
	if err := os.Remove(manifestPath); err != nil {
		t.Fatalf("failed removing manifest.json: %v", err)
	}

	rebuilt, err := thinkgraph.LoadOrRebuildManifest(filepath.Join(workspaceRoot, chat.ProjectID, "chats", chat.ChatID, "ledger"), "main")
	if err != nil {
		t.Fatalf("LoadOrRebuildManifest failed on >64KB line: %v", err)
	}
	if rebuilt.TotalEvents != 3 || rebuilt.EventCount != 1 || rebuilt.ActiveShard != "00001.jsonl" {
		t.Errorf("unexpected rebuilt manifest: %+v", rebuilt)
	}

	// Verify Rehydrator.Rebuild recovers all 3 events including the 128KB event
	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("failed initializing badger: %v", err)
	}
	defer graphDB.Close()

	rehydrator := thinkgraph.NewRehydrator(thinkgraph.NewGraphStore(graphDB, nil), workspaceRoot)
	if err := rehydrator.Rebuild(ctx, chat); err != nil {
		t.Fatalf("Rehydrator.Rebuild failed on large payload shard: %v", err)
	}

	timeline, err := graphDB.GetSubGraphTimeline(ctx, chat, chat.GraphID)
	if err != nil {
		t.Fatalf("GetSubGraphTimeline failed: %v", err)
	}
	if len(timeline) != 3 {
		t.Fatalf("expected 3 recovered nodes, got %d", len(timeline))
	}
	if len(timeline[1].Content) != len(largeContent) {
		t.Errorf("expected large event content length %d, got %d", len(largeContent), len(timeline[1].Content))
	}
}

func TestFileLedger_ConcurrentAppendSafety(t *testing.T) {
	ctx := context.Background()
	workspaceRoot := t.TempDir()
	ledger := thinkgraph.NewFileLedger(100, workspaceRoot)

	flowID := thinkgraph.NewGraphID(thinkgraph.PrefixFlow)
	chat := &thinkgraph.ChatAddress{
		ProjectID: "space",
		ChatID:    "chat-concurrent",
		SubGraph: thinkgraph.SubGraph{
			Name:    "flow-" + flowID.UUID.String(),
			GraphID: flowID,
		},
	}

	const numWriters = 8
	var wg sync.WaitGroup
	errs := make(chan error, numWriters)

	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ev := thinkgraph.NewEvent(thinkgraph.EventPrompt, fmt.Sprintf("concurrent-%d", idx))
			if err := ledger.AppendEvent(ctx, chat, ev); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("concurrent AppendEvent failed: %v", err)
	}

	ledgerDir := filepath.Join(workspaceRoot, chat.ProjectID, "chats", chat.ChatID, "ledger")
	manifest, err := thinkgraph.LoadOrRebuildManifest(ledgerDir, chat.Name)
	if err != nil {
		t.Fatalf("LoadOrRebuildManifest failed: %v", err)
	}
	if manifest.TotalEvents != numWriters {
		t.Errorf("expected manifest.TotalEvents == %d after concurrent appends, got %d", numWriters, manifest.TotalEvents)
	}
}

func TestFileLedger_DigestAndSpaceContextRoundTrip(t *testing.T) {
	ctx := context.Background()
	workspaceRoot := t.TempDir()
	ledger := thinkgraph.NewFileLedger(100, workspaceRoot)

	chat := &thinkgraph.ChatAddress{
		ProjectID: "space",
		ChatID:    "chat-artifacts",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}

	sumID1 := uuid.NewV7()
	sumID2 := uuid.NewV7()
	digest := thinkgraph.DigestMeta{
		ID:               uuid.NewV7(),
		Summary:          "Multi-line\ndigest summary",
		Tags:             []string{"arch", "go"},
		Promoted:         true,
		SummarizedEvents: []uuid.UUID{sumID1, sumID2},
	}
	if err := ledger.SaveDigest(ctx, chat, digest); err != nil {
		t.Fatalf("SaveDigest failed: %v", err)
	}

	sc := thinkgraph.SpaceContext{
		ID:      uuid.NewV7(),
		Content: "Always return wrapped errors.",
		Tags:    []string{"rules"},
	}
	if err := ledger.SaveSpaceContext(ctx, chat, sc); err != nil {
		t.Fatalf("SaveSpaceContext failed: %v", err)
	}

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("failed initializing badger: %v", err)
	}
	defer graphDB.Close()

	rehydrator := thinkgraph.NewRehydrator(thinkgraph.NewGraphStore(graphDB, nil), workspaceRoot)
	if err := rehydrator.Rebuild(ctx, chat); err != nil {
		t.Fatalf("Rebuild failed: %v", err)
	}

	digests, err := graphDB.GetDigests(ctx, chat)
	if err != nil {
		t.Fatalf("GetDigests failed: %v", err)
	}
	if len(digests) != 1 || digests[0].ID != digest.ID || !digests[0].Promoted || len(digests[0].SummarizedEvents) != 2 {
		t.Errorf("unexpected rehydrated digest: %+v", digests)
	}

	contexts, err := graphDB.GetSpaceContexts(ctx, chat)
	if err != nil {
		t.Fatalf("GetSpaceContexts failed: %v", err)
	}
	if len(contexts) != 1 || contexts[0].ID != sc.ID || contexts[0].Content != sc.Content {
		t.Errorf("unexpected rehydrated space context: %+v", contexts)
	}
}

func TestRehydrator_PropagatesCorruptShardError(t *testing.T) {
	ctx := context.Background()
	workspaceRoot := t.TempDir()
	chat := &thinkgraph.ChatAddress{
		ProjectID: "space",
		ChatID:    "chat-corrupt",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}

	logsDir := filepath.Join(workspaceRoot, chat.ProjectID, "chats", chat.ChatID, "ledger", "logs", "main")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(logsDir, "00000.jsonl"), []byte("{not valid json\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("failed initializing badger: %v", err)
	}
	defer graphDB.Close()

	rehydrator := thinkgraph.NewRehydrator(thinkgraph.NewGraphStore(graphDB, nil), workspaceRoot)
	err = rehydrator.Rebuild(ctx, chat)
	if err == nil {
		t.Fatalf("expected Rebuild to return an error on corrupt JSONL line, got nil")
	}
	if !strings.Contains(err.Error(), "00000.jsonl") {
		t.Errorf("expected error to identify corrupt shard file, got: %v", err)
	}
	_ = time.Now()
}
