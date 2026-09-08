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

// SubAgentBriefing encapsulates the explicitly distilled context handed down by the Manager.
type SubAgentBriefing struct {
	ContextDigest string
	Instruction   string
}

// SubAgentExecutor allows the Flow to trigger an LLM generation step.
// It receives ONLY the structured briefing and the physical Sandbox state, operating in a temporal vacuum.
type SubAgentExecutor func(ctx context.Context, briefing SubAgentBriefing, sandbox CandidateSandbox, agentID int, tokenChan chan<- AgentToken) error

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

// ChatMeta separates the stable directory ID from the mutable display name.
type ChatMeta struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// AppConfig defines the server's global capabilities and supported domain environments.
type AppConfig struct {
	SupportedDomains []string `json:"supported_domains"`
}

// SpaceState stores the persistent configuration for a physical ThinkSpace.
type SpaceState struct {
	Domain string `json:"domain"`
}
