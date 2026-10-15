package thinkgraph_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

func TestEventBus_And_LedgerSubscriber(t *testing.T) {
	workspaceRoot := t.TempDir()
	spaceID := "test-space"
	chatID := "test-thread"

	ledgerPath := filepath.Join(workspaceRoot, spaceID, "chats", chatID, "ledger")
	os.MkdirAll(filepath.Join(ledgerPath, "logs"), 0755)

	thread := &thinkgraph.Chat{
		ID:      chatID,
		SpaceID: spaceID,
	}

	eventBus := thinkgraph.NewEventBus()
	// Set threshold artificially low to force rollover
	ledgerSubscriber := thinkgraph.NewLedgerSubscriber(3, workspaceRoot)
	eventBus.Subscribe(ledgerSubscriber)

	for i := 1; i <= 5; i++ {
		event := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Test Message")
		event.Timestamp = time.Now().UTC()
		if err := eventBus.Publish(context.Background(), thread, event); err != nil {
			t.Fatalf("Failed to publish event: %v", err)
		}
	}

	// Verify shard rollover occurred
	_, err := os.Stat(filepath.Join(ledgerPath, "logs", "00000.jsonl"))
	if err != nil {
		t.Errorf("Expected shard 00000.jsonl to exist")
	}

	fileData, err := os.ReadFile(filepath.Join(ledgerPath, "logs", "00001.jsonl"))
	if err != nil {
		t.Fatalf("Expected shard 00001.jsonl to exist after rollover: %v", err)
	}

	if !strings.Contains(string(fileData), "Test Message") {
		t.Errorf("Rolled over ledger did not contain expected event content")
	}
}
