package workspace

import (
	"time"
	"uuid" // Native Go 1.27 stdlib
)

// LedgerEvent is the atomic, append-only unit of the chat history.
type LedgerEvent struct {
	ID        uuid.UUID `json:"id"` // Generated via uuid.NewV7()
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"` // e.g., "user_prompt", "tool_call", "digest_created"
	Payload   any       `json:"payload"`

	// Relational / Subgraph Data
	Tags     []string    `json:"tags,omitempty"`      // Contextual lenses (e.g., ["legals"])
	Refs     []uuid.UUID `json:"refs,omitempty"`      // UUIDs of past events this summarizes
	ParentID *uuid.UUID  `json:"parent_id,omitempty"` // For threading (e.g., tool response to a tool call)
}

// NewLedgerEvent creates a time-ordered event.
func NewLedgerEvent(eventType string, payload any) LedgerEvent {
	return LedgerEvent{
		ID:        uuid.NewV7(),
		Timestamp: time.Now(),
		Type:      eventType,
		Payload:   payload,
	}
}
