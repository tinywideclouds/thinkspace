package thinkgraph_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

func TestContextProjecter_Build(t *testing.T) {
	manifest := thinkgraph.NewChatManifest()

	stickyID := uuid.NewV7()
	manifest.Digests[stickyID] = thinkgraph.DigestMeta{
		ID:       stickyID,
		Summary:  "Always remember the user prefers idiomatic Go.",
		IsSticky: true,
	}

	time.Sleep(2 * time.Millisecond)
	focusDigestID := uuid.NewV7()
	manifest.Digests[focusDigestID] = thinkgraph.DigestMeta{
		ID:       focusDigestID,
		Summary:  "PostgreSQL connection pool configured in internal/db.",
		IsSticky: false,
	}

	ruleID := uuid.NewV7()
	manifest.SpaceContexts[ruleID] = thinkgraph.SpaceContext{
		ID:      ruleID,
		Content: "Use PostgreSQL exclusively.",
	}

	mistakeEvent := thinkgraph.NewEvent(thinkgraph.EventPrompt, "I want to use MySQL")
	mistakeNode := thinkgraph.Node{Event: mistakeEvent}

	amendingEvent := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Actually, switch to PostgreSQL")
	amendingNode := thinkgraph.Node{Event: amendingEvent}
	mistakeNode.Metadata["amended_by"] = amendingNode.ID.String()

	catalystID := uuid.NewV7()
	assemblyRequest := thinkgraph.ContextRequest{
		Manifest: manifest,
		RecentEvents: []thinkgraph.Node{
			mistakeNode,
			{Event: thinkgraph.NewEvent(thinkgraph.EventModel, "Previous answer")},
		},
		Catalyst: &thinkgraph.Node{
			Event: thinkgraph.Event{
				ID:      catalystID,
				Type:    thinkgraph.EventPrompt,
				Content: "Recent question",
			},
		},
		Focus: thinkgraph.AttentionFocus{
			PrimaryFocus:  []string{"golang"},
			DormantLenses: []string{"auth", "middleware"},
		},
		Amendments: map[string]thinkgraph.Node{
			amendingNode.ID.String(): amendingNode,
		},
	}

	projecter := thinkgraph.NewContextProjecter()
	trajectory, err := projecter.Build(context.Background(), assemblyRequest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(trajectory.LongTermMemory) != 4 {
		t.Fatalf("Expected 4 long-term memory items (1 metadata + 1 rule + 1 sticky + 1 focus digest), got %d", len(trajectory.LongTermMemory))
	}

	if !strings.Contains(trajectory.LongTermMemory[0], "[CONTEXT HORIZON METADATA]") {
		t.Errorf("Missing context horizon metadata in first slot")
	}
	if !strings.Contains(trajectory.LongTermMemory[0], "Dormant Memories Available via query_lens: auth, middleware") {
		t.Errorf("Missing dormant lenses metadata in first slot, got: %s", trajectory.LongTermMemory[0])
	}
	if trajectory.LongTermMemory[2] != "Always remember the user prefers idiomatic Go." {
		t.Errorf("Expected sticky digest in slot 2, got %q", trajectory.LongTermMemory[2])
	}
	if trajectory.LongTermMemory[3] != "PostgreSQL connection pool configured in internal/db." {
		t.Errorf("Expected non-sticky focus digest in slot 3, got %q", trajectory.LongTermMemory[3])
	}

	if len(trajectory.WorkingMemory) != 2 {
		t.Fatalf("Expected 2 working memory events, got %d", len(trajectory.WorkingMemory))
	}

	if !strings.Contains(trajectory.WorkingMemory[0].Content, "I want to use MySQL") {
		t.Errorf("Missing original mistaken prompt")
	}
	if !strings.Contains(trajectory.WorkingMemory[0].Content, "[System Note: This prompt was later amended to: Actually, switch to PostgreSQL]") {
		t.Errorf("Context projecter failed to inject the inline amendment")
	}

	if trajectory.Catalyst.Content != "Recent question" {
		t.Errorf("Missing recent catalyst context")
	}
}

func TestContextProjecter_RoleCollapsingAndFiltering(t *testing.T) {
	req := thinkgraph.ContextRequest{
		Manifest: thinkgraph.NewChatManifest(),
		RecentEvents: []thinkgraph.Node{
			{Event: thinkgraph.NewEvent(thinkgraph.EventPrompt, "First user prompt")},
			{Event: thinkgraph.NewEvent(thinkgraph.EventSystem, "System noise")},       // Should be ignored
			{Event: thinkgraph.NewEvent(thinkgraph.EventPrompt, "Second user prompt")}, // Should collapse
			{Event: thinkgraph.NewEvent(thinkgraph.EventModel, "")},                    // Empty shell, should be ignored
			{Event: thinkgraph.NewEvent(thinkgraph.EventModel, "Model reply 1")},
			{Event: thinkgraph.NewEvent(thinkgraph.EventModel, "Model reply 2")}, // Should collapse
		},
		Catalyst: &thinkgraph.Node{
			Event: thinkgraph.NewEvent(thinkgraph.EventPrompt, "Final catalyst prompt"),
		},
	}

	projecter := thinkgraph.NewContextProjecter()
	trajectory, err := projecter.Build(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(trajectory.WorkingMemory) != 2 {
		t.Fatalf("Expected exactly 2 working memory items due to collapsing and filtering, got %d", len(trajectory.WorkingMemory))
	}

	if trajectory.WorkingMemory[0].Role != "user" {
		t.Errorf("Expected first role to be user")
	}
	if !strings.Contains(trajectory.WorkingMemory[0].Content, "First user prompt") || !strings.Contains(trajectory.WorkingMemory[0].Content, "Second user prompt") {
		t.Errorf("User prompts were not collapsed correctly")
	}

	if trajectory.WorkingMemory[1].Role != "model" {
		t.Errorf("Expected second role to be model")
	}
	if !strings.Contains(trajectory.WorkingMemory[1].Content, "Model reply 1") || !strings.Contains(trajectory.WorkingMemory[1].Content, "Model reply 2") {
		t.Errorf("Model replies were not collapsed correctly")
	}
}

func TestContextProjecter_DefensiveIntegrityError(t *testing.T) {
	req := thinkgraph.ContextRequest{
		Manifest: thinkgraph.NewChatManifest(),
		RecentEvents: []thinkgraph.Node{
			{Event: thinkgraph.NewEvent(thinkgraph.EventPrompt, "User prompt")},
			{Event: thinkgraph.NewEvent(thinkgraph.EventModel, "Model reply")},
		},
		Catalyst: nil, // Missing catalyst implies the turn is handing off, but the history ends in Model
	}

	projecter := thinkgraph.NewContextProjecter()
	_, err := projecter.Build(context.Background(), req)
	if err == nil {
		t.Fatalf("Expected defensive error regarding invalid trajectory state, got nil")
	}
	if !strings.Contains(err.Error(), "invalid trajectory state") {
		t.Errorf("Expected specific invalid trajectory error, got: %v", err)
	}
}

func TestContextProjecter_NilManifestAndSystemNodeAnnotations(t *testing.T) {
	req := thinkgraph.ContextRequest{
		Manifest: nil,
		RecentEvents: []thinkgraph.Node{
			{Event: thinkgraph.NewEvent("flow_lifecycle", "Flow completed")},
			{Event: thinkgraph.NewEvent("candidate_resolution", "Proposal accepted")},
		},
		Catalyst: &thinkgraph.Node{
			Event: thinkgraph.NewEvent(thinkgraph.EventPrompt, "Next step"),
		},
	}

	projecter := thinkgraph.NewContextProjecter()
	trajectory, err := projecter.Build(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error with nil Manifest: %v", err)
	}

	if !strings.Contains(trajectory.Catalyst.Content, "[System: A sub-agent flow is executing or completed. Result: Flow completed]") {
		t.Errorf("expected flow_lifecycle annotation in collapsed catalyst, got: %q", trajectory.Catalyst.Content)
	}
	if !strings.Contains(trajectory.Catalyst.Content, "[System: The user resolved a proposal. Result: Proposal accepted]") {
		t.Errorf("expected candidate_resolution annotation in collapsed catalyst, got: %q", trajectory.Catalyst.Content)
	}
}
