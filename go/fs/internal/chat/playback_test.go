package chat_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/chat"
)

func TestPlaybackEngine_LoadState_PopulatesGraphAndManifest(t *testing.T) {
	tempDirectory := t.TempDir()
	ledgerPath := filepath.Join(tempDirectory, "conversation.jsonl")

	file, err := os.Create(ledgerPath)
	if err != nil {
		t.Fatalf("Failed to create temporary ledger: %v", err)
	}

	promptEvent := chat.NewEvent(chat.EventPrompt, "How do I build a server?")
	modelEvent := chat.NewEvent(chat.EventModel, "Use the standard library.")

	digestEvent := chat.NewEvent(chat.EventDigest, "User is building a Go standard library server.")
	digestEvent.Metadata = map[string]string{"is_sticky": "true"}

	events := []chat.Event{promptEvent, modelEvent, digestEvent}
	for _, event := range events {
		data, _ := json.Marshal(event)
		file.Write(append(data, '\n'))
	}
	file.Close()

	thread := &chat.Thread{
		ID:         "test-thread-123",
		LedgerPath: ledgerPath,
	}

	engine := chat.NewPlaybackEngine()
	graph, manifest, err := engine.LoadState(context.Background(), thread)
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	if len(graph.RecentEvents) != 2 {
		t.Errorf("Expected exactly 2 recent conversational events, got %d", len(graph.RecentEvents))
	}
	if graph.RecentEvents[0].Content != "How do I build a server?" {
		t.Errorf("Unexpected first event content: %s", graph.RecentEvents[0].Content)
	}

	if len(manifest.Digests) != 1 {
		t.Fatalf("Expected exactly 1 digest in manifest, got %d", len(manifest.Digests))
	}

	loadedDigest, exists := manifest.Digests[digestEvent.ID]
	if !exists {
		t.Errorf("Digest missing from manifest lookup map by ID")
	}
	if loadedDigest.Summary != "User is building a Go standard library server." {
		t.Errorf("Unexpected digest summary: %s", loadedDigest.Summary)
	}
	if !loadedDigest.IsSticky {
		t.Errorf("Expected digest to be flagged as sticky based on metadata")
	}
}

func TestPlaybackEngine_LoadState_HandlesMissingFile(t *testing.T) {
	thread := &chat.Thread{
		ID:         "new-thread",
		LedgerPath: "/path/does/not/exist.jsonl",
	}

	engine := chat.NewPlaybackEngine()
	graph, manifest, err := engine.LoadState(context.Background(), thread)

	if err != nil {
		t.Fatalf("Expected no error for missing file, got: %v", err)
	}
	if len(graph.RecentEvents) != 0 {
		t.Errorf("Expected empty graph for new thread")
	}
	if manifest.ThreadID != "new-thread" {
		t.Errorf("Expected manifest to correctly map to new-thread ID")
	}
}
