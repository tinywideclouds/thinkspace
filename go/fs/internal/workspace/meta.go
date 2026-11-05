package workspace

import (
	"context"
	"time"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
)

// AgentToken wraps a text chunk with its source agent ID for UI multiplexing.
type AgentToken struct {
	AgentID int
	Text    string
}

// SubAgentExecutor allows the Flow to trigger an LLM generation step.
// It receives the strictly typed cognitive state (ThinkBundle) and the physical Sandbox state.
type SubAgentExecutor func(ctx context.Context, bundle cognition.ThinkBundle, sandbox CandidateSandbox, agentID int, tokenChan chan<- AgentToken) error

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

// BranchMeta tracks the topology of a specific timeline fork.
type BranchMeta struct {
	ParentBranch string `json:"parent_branch"`
	ForkPoint    string `json:"fork_point"` // Event ID where divergence occurred
}

// ChatMeta separates the stable directory ID from the mutable display name.
type ChatMeta struct {
	ID                     string    `json:"id"`
	Name                   string    `json:"name"`
	CreatedAt              time.Time `json:"created_at"`
	ActiveThinkState       string    `json:"active_think_state"`       // e.g., "concept" or "golang"
	InitializedThinkStates []string  `json:"initialized_think_states"` // e.g., ["concept", "golang"]

	// Topology Routing
	ActiveBranch string                `json:"active_branch"`
	Branches     map[string]BranchMeta `json:"branches"`

	// Memory Sliding Window State
	DigestionHorizon  uuid.UUID `json:"digestion_horizon,omitempty"`  // Last event successfully digested
	EvaluationHorizon uuid.UUID `json:"evaluation_horizon,omitempty"` // Last event evaluated by the background worker
}

// AppConfig defines the server's global capabilities and supported domain environments.
type AppConfig struct {
	SupportedDomains []string `json:"supported_domains"`
}

// SpaceState stores the persistent environmental configuration for a physical ThinkSpace.
type SpaceState struct {
	ThinkState string `json:"think_state"` // e.g., "concept" or "golang"
}

// ReviewPolicySettings controls the human-in-the-loop context review heuristic.
type ReviewPolicySettings struct {
	Active         bool `json:"active"`
	MinTokens      int  `json:"min_tokens"`
	IncludePersona bool `json:"include_persona"`
}

// BundleSettings controls domain-specific prompt and context assembly rules.
type BundleSettings struct {
	MaxWorkingMemoryEvents int  `json:"max_working_memory_events,omitempty"`
	IncludeMapbookOnBoot   bool `json:"include_mapbook_on_boot"`
}

// SpaceSettings stores user workflow and UI preferences for a physical ThinkSpace.
type SpaceSettings struct {
	ReviewPolicy ReviewPolicySettings `json:"review_policy"`
	Bundle       BundleSettings       `json:"bundle,omitempty"`
}
