package thinkgraph

import (
	"time"
	"uuid"
)

const (
	EventPrompt     = "prompt"
	EventModel      = "model"
	EventTool       = "tool"
	EventCandidate  = "candidate"
	EventResolution = "resolution"
	EventDigest     = "digest" // <-- Restored
)

// Event represents a single immutable beat in the chronological ledger.
type Event struct {
	ID        uuid.UUID         `json:"id"`
	Type      string            `json:"type"`
	Timestamp time.Time         `json:"timestamp"`
	Content   string            `json:"content"`
	Tags      []string          `json:"tags,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

func NewEvent(eventType, content string) Event {
	return Event{
		ID:        uuid.NewV7(),
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Content:   content,
	}
}
