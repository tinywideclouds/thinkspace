package thinkgraph

import (
	"encoding/json"
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
	MutationUpdate    MutationType = "update"
)

type MutationEvent struct {
	TargetID  uuid.UUID
	Type      MutationType
	Timestamp time.Time
	Payload   string          // Legacy content text updates
	Data      json.RawMessage // Strongly typed data updates
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