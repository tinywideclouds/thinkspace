package chat_test

import (
	"strings"
	"testing"
	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/chat"
)

func TestAssembler_Build(t *testing.T) {
	manifest := chat.NewThreadManifest("test-thread")

	stickyID := uuid.NewV7()
	manifest.Digests[stickyID] = chat.DigestMeta{
		ID:       stickyID,
		Summary:  "Always remember the user prefers idiomatic Go.",
		IsSticky: true,
	}

	manifest.Lenses["routing"] = []uuid.UUID{uuid.NewV7()}

	assemblyRequest := chat.AssemblyRequest{
		Manifest:     manifest,
		ActiveLenses: []string{},
		RecentEvents: []chat.Event{
			chat.NewEvent(chat.EventPrompt, "Recent question"),
		},
	}

	assembler := chat.NewAssembler()
	contextHistory, err := assembler.Build(assemblyRequest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(contextHistory) != 3 {
		t.Fatalf("Expected 3 content blocks (sticky + tags + recent event), got %d", len(contextHistory))
	}

	if !strings.Contains(contextHistory[0].Parts[0].Text, "idiomatic Go") {
		t.Errorf("Missing sticky context")
	}
	if !strings.Contains(contextHistory[1].Parts[0].Text, "Available Conceptual Tags") || !strings.Contains(contextHistory[1].Parts[0].Text, "routing") {
		t.Errorf("Missing available conceptual tags projection")
	}
	if contextHistory[2].Parts[0].Text != "Recent question" {
		t.Errorf("Missing recent event context")
	}
}

func TestAssembler_Build_TruncatesTrajectory(t *testing.T) {
	manifest := chat.NewThreadManifest("test-thread")

	var events []chat.Event
	for i := 0; i < 10; i++ {
		events = append(events, chat.NewEvent(chat.EventPrompt, "msg"))
	}

	assemblyRequest := chat.AssemblyRequest{
		Manifest:     manifest,
		ActiveLenses: []string{},
		RecentEvents: events,
	}

	assembler := chat.NewAssembler()
	contextHistory, err := assembler.Build(assemblyRequest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1 system tag message + 5 truncated recent events
	if len(contextHistory) != 6 {
		t.Fatalf("Expected exactly 6 content blocks due to sliding window truncation, got %d", len(contextHistory))
	}
}
