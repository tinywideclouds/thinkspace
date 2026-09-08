package chat_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/chat"
)

func TestEventBus_And_LedgerSubscriber(t *testing.T) {
	tempDir := t.TempDir()
	ledgerPath := filepath.Join(tempDir, "conversation.jsonl")

	thread := &chat.Thread{
		ID:         "test-thread",
		LedgerPath: ledgerPath,
	}

	eventBus := chat.NewEventBus()
	ledgerSubscriber := chat.NewLedgerSubscriber()
	eventBus.Subscribe(ledgerSubscriber)

	event := chat.NewEvent(chat.EventPrompt, "Test Message")
	if err := eventBus.Publish(context.Background(), thread, event); err != nil {
		t.Fatalf("Failed to publish event: %v", err)
	}

	fileData, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("Failed to read ledger file: %v", err)
	}

	if !strings.Contains(string(fileData), "Test Message") {
		t.Errorf("Ledger file did not contain expected event content")
	}
}
