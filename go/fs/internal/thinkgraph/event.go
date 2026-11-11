package thinkgraph

import (
	"context"
	"encoding/json"
	"time"

	"uuid"
)

type EventType string

const (
	EventPrompt       EventType = "prompt"
	EventModel        EventType = "model"
	EventDigest       EventType = "digest"
	EventBranchAnchor EventType = "branch_anchor"
	EventSystem       EventType = "system"
)

// Event is the storage envelope for the append-only ledger.
type Event struct {
	ID        uuid.UUID         `json:"id"`
	Type      EventType         `json:"type"`
	Timestamp time.Time         `json:"timestamp"`
	Content   string            `json:"content"`
	Tags      []string          `json:"tags,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Edges     map[string]string `json:"edges,omitempty"`
	GraphID   GraphID           `json:"graph_id,omitempty"`
	Data      json.RawMessage   `json:"data,omitempty"` // Strictly typed domain structs live here
}

func NewEvent(eventType EventType, content string) Event {
	id := uuid.NewV7()
	return Event{
		ID:        id,
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Content:   content,
		Metadata:  make(map[string]string),
	}
}

// EventWriter defines the shared write contract for both the physical ledger and the materialized graph.
type EventWriter interface {
	AppendEvent(ctx context.Context, c *ChatAddress, event Event) error
	SaveDigest(ctx context.Context, c *ChatAddress, digest DigestMeta) error
	SaveSpaceContext(ctx context.Context, c *ChatAddress, spaceCtx SpaceContext) error
	SaveMutation(ctx context.Context, c *ChatAddress, mutation MutationEvent) error
}
