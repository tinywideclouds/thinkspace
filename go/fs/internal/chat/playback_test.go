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

	// Phase 3 Coverage: Tagged Event
	tagEvent := chat.NewEvent(chat.EventCandidate, "Tagged proposal")
	tagEvent.Tags = []string{"architecture", "auth"}

	events := []chat.Event{promptEvent, modelEvent, digestEvent, tagEvent}
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

	if len(graph.RecentEvents) != 3 {
		t.Errorf("Expected exactly 3 recent conversational events, got %d", len(graph.RecentEvents))
	}

	// Assert Lenses Indexing
	if len(manifest.Lenses["architecture"]) != 1 || manifest.Lenses["architecture"][0] != tagEvent.ID {
		t.Errorf("Expected 'architecture' lens to correctly index the tagEvent ID")
	}
	if len(manifest.Lenses["auth"]) != 1 || manifest.Lenses["auth"][0] != tagEvent.ID {
		t.Errorf("Expected 'auth' lens to correctly index the tagEvent ID")
	}

	if len(manifest.Digests) != 1 {
		t.Fatalf("Expected exactly 1 digest in manifest, got %d", len(manifest.Digests))
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
