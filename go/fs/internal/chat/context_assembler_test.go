package chat_test

import (
	"strings"
	"testing"
	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/chat"
)

func TestContextAssembler_Build(t *testing.T) {
	manifest := chat.NewThreadManifest("test-thread")

	stickyID := uuid.NewV7()
	manifest.Digests[stickyID] = chat.DigestMeta{
		ID:       stickyID,
		Summary:  "Always remember the user prefers idiomatic Go.",
		IsSticky: true,
	}

	assemblyRequest := chat.ContextAssemblyRequest{
		Manifest:     manifest,
		ActiveLenses: []string{},
		RecentEvents: []chat.Event{
			chat.NewEvent(chat.EventPrompt, "Recent question"),
		},
	}

	assembler := chat.NewContextAssembler()
	contextHistory, err := assembler.Build(assemblyRequest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(contextHistory) != 2 {
		t.Fatalf("Expected 2 content blocks (1 sticky + 1 recent event), got %d", len(contextHistory))
	}

	if !strings.Contains(contextHistory[0].Parts[0].Text, "idiomatic Go") {
		t.Errorf("Missing sticky context")
	}
	if contextHistory[1].Parts[0].Text != "Recent question" {
		t.Errorf("Missing recent event context")
	}
}
