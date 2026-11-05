package thinkgraph

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"uuid"
)

type GraphPrefix string

const (
	PrefixChat    GraphPrefix = "c"
	PrefixFlow    GraphPrefix = "f"
	PrefixFeature GraphPrefix = "ft"
)

// GraphID strongly types our DAG identities while retaining native UUID v7 sorting and operations.
type GraphID struct {
	Prefix GraphPrefix
	UUID   uuid.UUID
}

// NewGraphID generates a time-ordered v7 UUID under a specific domain prefix.
func NewGraphID(prefix GraphPrefix) GraphID {
	return GraphID{
		Prefix: prefix,
		UUID:   uuid.NewV7(),
	}
}

// ParseGraphID safely reconstructs a GraphID from its string representation.
func ParseGraphID(s string) (GraphID, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return GraphID{}, fmt.Errorf("invalid GraphID format: missing prefix delimiter")
	}

	parsedUUID, err := uuid.Parse(parts[1])
	if err != nil {
		return GraphID{}, fmt.Errorf("invalid UUID payload in GraphID: %w", err)
	}

	return GraphID{
		Prefix: GraphPrefix(parts[0]),
		UUID:   parsedUUID,
	}, nil
}

// String implements the fmt.Stringer interface (e.g., "f:018e6b...").
func (g GraphID) String() string {
	return fmt.Sprintf("%s:%s", g.Prefix, g.UUID.String())
}

// IsZero allows for easy empty-state checking.
func (g GraphID) IsZero() bool {
	var empty uuid.UUID
	return g.UUID == empty
}

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
