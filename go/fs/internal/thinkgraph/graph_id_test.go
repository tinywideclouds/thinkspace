package thinkgraph_test

import (
	"encoding/json"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

func TestGraphID_ParseAndJSONRoundTrip(t *testing.T) {
	id := thinkgraph.NewGraphID(thinkgraph.PrefixFlow)
	if id.IsZero() {
		t.Fatalf("expected non-zero GraphID")
	}

	encoded, err := json.Marshal(id)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded thinkgraph.GraphID
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if decoded != id {
		t.Errorf("expected round-tripped GraphID %s, got %s", id, decoded)
	}

	for _, invalid := range []string{"", "no-colon", ":018e6b00-0000-7000-8000-000000000000", "f:not-a-uuid"} {
		if _, parseErr := thinkgraph.ParseGraphID(invalid); parseErr == nil {
			t.Errorf("expected ParseGraphID(%q) to fail, got nil", invalid)
		}
	}
}

func TestUnmarshalMutation_AllVariantsAndErrors(t *testing.T) {
	ev := thinkgraph.NewEvent(thinkgraph.EventPrompt, "base")
	base := thinkgraph.BaseMutation{TargetID: ev.ID}

	variants := []thinkgraph.Mutator{
		thinkgraph.TombstoneMutation{BaseMutation: thinkgraph.BaseMutation{TargetID: base.TargetID, Type: thinkgraph.MutationTombstone}},
		thinkgraph.RestoreMutation{BaseMutation: thinkgraph.BaseMutation{TargetID: base.TargetID, Type: thinkgraph.MutationRestore}},
		thinkgraph.ContentMutation{BaseMutation: thinkgraph.BaseMutation{TargetID: base.TargetID, Type: thinkgraph.MutationContent}, Content: "edited"},
		thinkgraph.StateMutation{BaseMutation: thinkgraph.BaseMutation{TargetID: base.TargetID, Type: thinkgraph.MutationState}, State: json.RawMessage(`{"ok":true}`)},
		thinkgraph.MetadataMutation{BaseMutation: thinkgraph.BaseMutation{TargetID: base.TargetID, Type: thinkgraph.MutationMetadata}, Metadata: map[string]string{"k": "v"}},
	}

	for _, v := range variants {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("failed marshaling %T: %v", v, err)
		}
		got, err := thinkgraph.UnmarshalMutation(raw)
		if err != nil {
			t.Fatalf("UnmarshalMutation failed for %T: %v", v, err)
		}
		if got.EventType() != v.EventType() || got.Target() != v.Target() {
			t.Errorf("mismatched mutation round-trip: got %+v, want %+v", got, v)
		}
	}

	if _, err := thinkgraph.UnmarshalMutation([]byte(`{"type":"unknown_type"}`)); err == nil {
		t.Errorf("expected error for unknown mutation type")
	}
}
