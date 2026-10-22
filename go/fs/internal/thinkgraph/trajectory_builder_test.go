package thinkgraph_test

import (
	"context"
	"strings"
	"testing"
	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

func TestAssembler_Build(t *testing.T) {
	manifest := thinkgraph.NewChatManifest()

	stickyID := uuid.NewV7()
	manifest.Digests[stickyID] = thinkgraph.DigestMeta{
		ID:       stickyID,
		Summary:  "Always remember the user prefers idiomatic Go.",
		Promoted: true,
	}

	ruleID := uuid.NewV7()
	manifest.SpaceContexts[ruleID] = thinkgraph.SpaceContext{
		ID:      ruleID,
		Content: "Use PostgreSQL exclusively.",
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

	if len(trajectory.LongTermMemory) != 3 {
		t.Fatalf("Expected 3 long-term memory items (1 metadata + 1 sticky + 1 rule), got %d", len(trajectory.LongTermMemory))
	}

	// Assert the new Contextual Horizon metadata was injected first
	if !strings.Contains(trajectory.LongTermMemory[0], "[CONTEXT HORIZON METADATA]") {
		t.Errorf("Missing context horizon metadata in first slot")
	}
	if !strings.Contains(trajectory.LongTermMemory[0], "Active Topic Focus: golang") {
		t.Errorf("Metadata did not properly format the AttentionFocus tags")
	}

	// Assert the context rule memory was appended
	if !strings.Contains(trajectory.LongTermMemory[1], "Use PostgreSQL") {
		t.Errorf("Missing SpaceContext rule in second slot")
	}

	// Assert the sticky digest was appended
	if !strings.Contains(trajectory.LongTermMemory[2], "idiomatic Go") {
		t.Errorf("Missing promoted digest in third slot")
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
