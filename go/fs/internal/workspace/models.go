package workspace

import (
	"context"
	"time"
)

// AgentToken wraps a text chunk with its source agent ID for UI multiplexing.
type AgentToken struct {
	AgentID int
	Text    string
}

// SubAgentExecutor allows the Flow to trigger an LLM generation step without
// knowing the implementation details of the LLM provider.
// It now accepts a channel to stream tokens back to the UI.
type SubAgentExecutor func(ctx context.Context, instructions string, sandboxDir string, agentID int, tokenChan chan<- AgentToken) error

// Thread represents the "Slow" branch, our accepted ledger of reality.
type Thread struct {
	ID         string
	Branch     string // e.g., "chat/quantum-sim"
	LedgerPath string // Absolute path to conversation.jsonl
	Dir        string // Absolute path to the thread's working directory
}

// Candidate represents a "Fast" branch, holding unverified tool outputs.
type Candidate struct {
	ID        string
	ThreadID  string
	Branch    string // e.g., "candidate/quantum-sim-1723549800"
	CommitSHA string
	Status    CandidateStatus
	CreatedAt time.Time
}

type CandidateStatus string

const (
	StatusPending  CandidateStatus = "pending"
	StatusAccepted CandidateStatus = "accepted"
	StatusRejected CandidateStatus = "rejected"
)

// EventType defines the nature of a ledger entry.
type EventType string

const (
	EventPrompt     EventType = "prompt"
	EventModel      EventType = "model"
	EventTool       EventType = "tool_call"
	EventCandidate  EventType = "candidate_proposed"
	EventResolution EventType = "candidate_resolved"
)

// Event is a single line in the append-only conversation.jsonl.
type Event struct {
	ID        string            `json:"id"`
	Timestamp time.Time         `json:"timestamp"`
	Type      EventType         `json:"type"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}
