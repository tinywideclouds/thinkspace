package thinkgraph

import (
	"encoding/json"
	"fmt"
	"time"
	"uuid"
)

// Node is the materialized read-model for BadgerDB.
type Node struct {
	Event
	Tombstoned bool `json:"tombstoned"`

	// MaterializedState acts as a projection data cache held directly on the graph for speed.
	// It stores heavy, enriched artifacts (such as parsed Git diffs or structural states)
	// that were fetched once during ingestion by the GraphBuilder. This prevents consumers
	// from repeating expensive I/O operations on every read.
	MaterializedState json.RawMessage `json:"materialized_state,omitempty"`
}

type MutationType string

const (
	MutationTombstone MutationType = "tombstone"
	MutationRestore   MutationType = "restore"
	MutationContent   MutationType = "content"
	MutationState     MutationType = "state"
	MutationMetadata  MutationType = "metadata"
	MutationTags      MutationType = "tags"
)

// Mutator defines the polymorphic interface for all graph projection updates.
type Mutator interface {
	Target() uuid.UUID
	EventType() MutationType
	Time() time.Time
}

type BaseMutation struct {
	TargetID  uuid.UUID    `json:"target_id"`
	Type      MutationType `json:"type"`
	Timestamp time.Time    `json:"timestamp"`
}

func (b BaseMutation) Target() uuid.UUID       { return b.TargetID }
func (b BaseMutation) EventType() MutationType { return b.Type }
func (b BaseMutation) Time() time.Time         { return b.Timestamp }

// TombstoneMutation marks a node as deleted without removing it from the ledger.
type TombstoneMutation struct {
	BaseMutation
}

// RestoreMutation un-marks a node as deleted.
type RestoreMutation struct {
	BaseMutation
}

// ContentMutation explicitly targets the Content field (e.g., for inline edits).
type ContentMutation struct {
	BaseMutation
	Content string `json:"content"`
}

// StateMutation targets the MaterializedState field (and optionally Content) in a single atomic update.
type StateMutation struct {
	BaseMutation
	Content string          `json:"content,omitempty"`
	State   json.RawMessage `json:"state"`
}

// MetadataMutation explicitly targets the key-value metadata map (e.g., for transition badges).
type MetadataMutation struct {
	BaseMutation
	Metadata map[string]string `json:"metadata"`
}

// TagsMutation updates the semantic tags attached to a graph node and its lens indices.
type TagsMutation struct {
	BaseMutation
	Tags []string `json:"tags"`
}

// UnmarshalMutation acts as a factory to rehydrate the correct concrete Mutator from disk.
func UnmarshalMutation(data []byte) (Mutator, error) {
	var base BaseMutation
	if err := json.Unmarshal(data, &base); err != nil {
		return nil, err
	}

	switch base.Type {
	case MutationTombstone:
		var m TombstoneMutation
		err := json.Unmarshal(data, &m)
		return m, err
	case MutationRestore:
		var m RestoreMutation
		err := json.Unmarshal(data, &m)
		return m, err
	case MutationContent:
		var m ContentMutation
		err := json.Unmarshal(data, &m)
		return m, err
	case MutationState:
		var m StateMutation
		err := json.Unmarshal(data, &m)
		return m, err
	case MutationMetadata:
		var m MetadataMutation
		err := json.Unmarshal(data, &m)
		return m, err
	case MutationTags:
		var m TagsMutation
		err := json.Unmarshal(data, &m)
		return m, err
	default:
		return nil, fmt.Errorf("unknown mutation type: %s", base.Type)
	}
}

// AttentionFocus models the cognitive focus of the immediate conversational trajectory.
type AttentionFocus struct {
	PrimaryFocus   []string
	RelatedContext []string
	DormantLenses  []string
}

// GraphData represents the backend's reconstructed memory state loaded from disk.
type GraphData struct {
	Digests  []DigestMeta
	Timeline []Node
	Focus    AttentionFocus
}
