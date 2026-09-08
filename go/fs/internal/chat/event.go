package chat

import (
	"time"
	"uuid"
)

type EventType string

const (
	EventPrompt     EventType = "prompt"
	EventModel      EventType = "model"
	EventTool       EventType = "tool_call"
	EventCandidate  EventType = "candidate_proposed"
	EventResolution EventType = "candidate_resolved"
	EventDigest     EventType = "digest_created"
)

// Event is the atomic, append-only unit of the chat history.
type Event struct {
	ID        uuid.UUID         `json:"id"`
	Timestamp time.Time         `json:"timestamp"`
	Type      EventType         `json:"type"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`

	// Relational / Subgraph Data
	Tags     []string    `json:"tags,omitempty"`
	Refs     []uuid.UUID `json:"refs,omitempty"`
	ParentID *uuid.UUID  `json:"parent_id,omitempty"`
}

func NewEvent(eventType EventType, content string) Event {
	return Event{
		ID:        uuid.NewV7(),
		Timestamp: time.Now().UTC(),
		Type:      eventType,
		Content:   content,
	}
}
