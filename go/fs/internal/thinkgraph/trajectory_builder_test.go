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

	mistakeEvent := thinkgraph.NewEvent(thinkgraph.EventPrompt, "I want to use MySQL")
	amendingEvent := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Actually, switch to PostgreSQL")
	mistakeEvent.Metadata["amended_by"] = amendingEvent.ID.String()

	assemblyRequest := thinkgraph.TrajectoryRequest{
		Manifest: manifest,
		RecentEvents: []thinkgraph.Event{
			mistakeEvent,
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
		Amendments: map[string]thinkgraph.Event{
			amendingEvent.ID.String(): amendingEvent,
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

	if !strings.Contains(trajectory.LongTermMemory[0], "[CONTEXT HORIZON METADATA]") {
		t.Errorf("Missing context horizon metadata in first slot")
	}

	if len(trajectory.WorkingMemory) != 2 {
		t.Fatalf("Expected 2 working memory events, got %d", len(trajectory.WorkingMemory))
	}

	// Verify the semantic inline correction was applied
	if !strings.Contains(trajectory.WorkingMemory[0].Content, "I want to use MySQL") {
		t.Errorf("Missing original mistaken prompt")
	}
	if !strings.Contains(trajectory.WorkingMemory[0].Content, "[System Note: This prompt was later amended to: Actually, switch to PostgreSQL]") {
		t.Errorf("Trajectory builder failed to inject the inline amendment")
	}

	if trajectory.Catalyst.Content != "Recent question" {
		t.Errorf("Missing recent catalyst context")
	}
}
