package thinkgraph_test

import (
	"context"
	"strings"
	"testing"
	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

func TestAssembler_Build(t *testing.T) {
	manifest := thinkgraph.NewChatManifest("test-thread")

	stickyID := uuid.NewV7()
	manifest.Digests[stickyID] = thinkgraph.DigestMeta{
		ID:       stickyID,
		Summary:  "Always remember the user prefers idiomatic Go.",
		IsSticky: true,
	}

	assemblyRequest := thinkgraph.TrajectoryRequest{
		Manifest: manifest,
		RecentEvents: []thinkgraph.Event{
			thinkgraph.NewEvent(thinkgraph.EventModel, "Previous answer"),
		},
		Catalyst: &thinkgraph.Event{
			ID:      uuid.NewV7(),
			Type:    thinkgraph.EventPrompt,
			Content: "Recent question",
		},
		Focus: thinkgraph.AttentionFocus{
			PrimaryFocus: []string{"golang"},
		},
	}

	a := thinkgraph.NewTrajectoryBuilder()
	trajectory, err := a.Build(context.Background(), assemblyRequest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(trajectory.LongTermMemory) != 2 {
		t.Fatalf("Expected 2 long-term memory items (1 metadata + 1 sticky), got %d", len(trajectory.LongTermMemory))
	}

	// Assert the new Contextual Horizon metadata was injected first
	if !strings.Contains(trajectory.LongTermMemory[0], "[CONTEXT HORIZON METADATA]") {
		t.Errorf("Missing context horizon metadata in first slot")
	}
	if !strings.Contains(trajectory.LongTermMemory[0], "Active Topic Focus: golang") {
		t.Errorf("Metadata did not properly format the AttentionFocus tags")
	}

	// Assert the sticky memory was appended after the metadata
	if !strings.Contains(trajectory.LongTermMemory[1], "idiomatic Go") {
		t.Errorf("Missing sticky context in second slot")
	}

	if len(trajectory.WorkingMemory) != 1 {
		t.Fatalf("Expected 1 working memory event, got %d", len(trajectory.WorkingMemory))
	}
	if trajectory.WorkingMemory[0].Role != "model" {
		t.Errorf("Expected role 'model', got %s", trajectory.WorkingMemory[0].Role)
	}

	if trajectory.Catalyst.Content != "Recent question" {
		t.Errorf("Missing recent catalyst context")
	}
	if trajectory.Catalyst.Role != "user" {
		t.Errorf("Expected catalyst role 'user', got %s", trajectory.Catalyst.Role)
	}
}
