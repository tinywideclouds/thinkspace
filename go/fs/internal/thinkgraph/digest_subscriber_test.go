package thinkgraph_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

func TestDigestSubscriber_WritesMarkdownFile(t *testing.T) {
	workspaceRoot := t.TempDir()
	spaceID := "test-space"
	chatID := "test-thread"
	ledgerPath := filepath.Join(workspaceRoot, spaceID, "chats", chatID, "ledger")

	thread := &thinkgraph.Chat{
		ID:      chatID,
		SpaceID: spaceID,
	}

	eventBus := thinkgraph.NewEventBus()
	digestSub := thinkgraph.NewDigestSubscriber(workspaceRoot)
	ledgerSub := thinkgraph.NewLedgerSubscriber(1000, workspaceRoot)
	eventBus.Subscribe(digestSub)
	eventBus.Subscribe(ledgerSub)

	event := thinkgraph.NewEvent(thinkgraph.EventDigest, "Critical architectural decision.")
	event.Metadata = map[string]string{"is_sticky": "true"}
	event.Tags = []string{"architecture", "auth"}

	if err := eventBus.Publish(context.Background(), thread, event); err != nil {
		t.Fatalf("Failed to publish event: %v", err)
	}

	// Verify markdown file generation in ledger/digests/
	digestPath := filepath.Join(ledgerPath, "digests", event.ID.String()+".md")
	fileData, err := os.ReadFile(digestPath)
	if err != nil {
		t.Fatalf("Failed to read generated digest markdown at %s: %v", digestPath, err)
	}

	fileStr := string(fileData)
	if !strings.Contains(fileStr, "sticky: true") {
		t.Errorf("Markdown missing sticky frontmatter")
	}
	if !strings.Contains(fileStr, `tags: ["architecture", "auth"]`) {
		t.Errorf("Markdown missing tags frontmatter")
	}
	if !strings.Contains(fileStr, "Critical architectural decision.") {
		t.Errorf("Markdown missing body content")
	}

	// Verify EventDigest was NOT written to logs/ shard
	shardPath := filepath.Join(ledgerPath, "logs", "00000.jsonl")
	if _, err := os.Stat(shardPath); err == nil {
		logData, _ := os.ReadFile(shardPath)
		if strings.Contains(string(logData), event.ID.String()) {
			t.Errorf("LedgerSubscriber incorrectly wrote digest to .jsonl shard")
		}
	}
}
