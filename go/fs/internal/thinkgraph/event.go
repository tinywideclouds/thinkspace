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
	EventDigest     = "digest"
	EventSystem     = "system"
)

type MutationType string

const (
	MutationTombstone MutationType = "tombstone"
	MutationRestore   MutationType = "restore"
	MutationUpdate    MutationType = "update"
)

type MutationEvent struct {
	TargetID  uuid.UUID    `json:"target_id"`
	Type      MutationType `json:"type"`
	Timestamp time.Time    `json:"timestamp"`
	Payload   string       `json:"payload,omitempty"`
}

type Event struct {
	ID         uuid.UUID         `json:"id"`
	Type       string            `json:"type"`
	Timestamp  time.Time         `json:"timestamp"`
	Content    string            `json:"content"`
	Tags       []string          `json:"tags,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	Tombstoned bool              `json:"tombstoned"`
}

func NewEvent(eventType, content string) Event {
	return Event{
		ID:        uuid.NewV7(),
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Content:   content,
		Tags:      []string{},
		Metadata:  make(map[string]string),
	}
}

func (e Event) IsTombstoned() bool {
	return e.Tombstoned
}
