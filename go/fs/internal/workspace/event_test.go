package workspace_test

import (
	"strings"
	"testing"
	"time"
	"uuid" // Native Go 1.27 stdlib

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

func TestLedgerEvent_TimeOrderingV7(t *testing.T) {
	// 1. Generate events sequentially
	event1 := workspace.NewLedgerEvent("user_prompt", "Hello")

	// Sleep for 2ms to ensure the millisecond prefix in the V7 UUID ticks forward
	time.Sleep(2 * time.Millisecond)
	event2 := workspace.NewLedgerEvent("tool_call", "Doing work")

	time.Sleep(2 * time.Millisecond)
	event3 := workspace.NewLedgerEvent("digest_created", "Summary")

	// 2. Verify Lexicographical Sorting
	// Because UUIDv7 puts the timestamp in the most significant bits, standard
	// string sorting perfectly matches chronological creation order.
	if !(event1.ID.String() < event2.ID.String()) {
		t.Errorf("Expected event1 ID to sort before event2 ID. Got: %s >= %s", event1.ID, event2.ID)
	}
	if !(event2.ID.String() < event3.ID.String()) {
		t.Errorf("Expected event2 ID to sort before event3 ID. Got: %s >= %s", event2.ID, event3.ID)
	}

	t.Logf("Event 1 ID: %s", event1.ID.String())
	t.Logf("Event 2 ID: %s", event2.ID.String())
	t.Logf("Event 3 ID: %s", event3.ID.String())
}

func TestContextAssembler_BuildLenses(t *testing.T) {
	manifest := workspace.NewThreadManifest("test-thread")

	// Setup: Create some mock digests with new UUIDv7s
	stickyID := uuid.NewV7()
	manifest.Digests[stickyID] = workspace.DigestMeta{
		ID:       stickyID,
		Summary:  "Always remember the user prefers idiomatic Go.",
		IsSticky: true,
	}

	legalID := uuid.NewV7()
	manifest.Digests[legalID] = workspace.DigestMeta{
		ID:       legalID,
		Summary:  "Do not use GPL licensed code in this project.",
		IsSticky: false,
	}

	perfID := uuid.NewV7()
	manifest.Digests[perfID] = workspace.DigestMeta{
		ID:       perfID,
		Summary:  "Avoid heap allocations inside the hot loop.",
		IsSticky: false,
	}

	// Setup: Map the Lenses (Subgraphs)
	manifest.Lenses["legals"] = []uuid.UUID{legalID}
	manifest.Lenses["performance"] = []uuid.UUID{perfID}

	assembler := &workspace.ContextAssembler{}

	t.Run("Base Context (Sticky Only)", func(t *testing.T) {
		req := workspace.AssemblyRequest{
			Manifest:     manifest,
			ActiveLenses: []string{}, // No tags selected
		}

		history, err := assembler.Build(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(history) != 1 {
			t.Fatalf("Expected exactly 1 content block (sticky), got %d", len(history))
		}
		if !strings.Contains(history[0].Parts[0].Text, "idiomatic Go") {
			t.Errorf("Missing sticky context")
		}
	})

	t.Run("Context with Performance Lens", func(t *testing.T) {
		req := workspace.AssemblyRequest{
			Manifest:     manifest,
			ActiveLenses: []string{"performance"}, // Activate performance tag
		}

		history, _ := assembler.Build(req)

		if len(history) != 2 {
			t.Fatalf("Expected 2 content blocks (sticky + perf), got %d", len(history))
		}

		// History should contain both the sticky digest and the performance digest
		combinedText := history[0].Parts[0].Text + history[1].Parts[0].Text
		if !strings.Contains(combinedText, "idiomatic Go") || !strings.Contains(combinedText, "heap allocations") {
			t.Errorf("Missing expected context in combined payload: %s", combinedText)
		}
		if strings.Contains(combinedText, "GPL licensed") {
			t.Errorf("Legals context leaked into the performance lens!")
		}
	})
}
