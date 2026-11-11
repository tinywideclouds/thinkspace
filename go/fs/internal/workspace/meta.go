package workspace

import (
	"time"

	"uuid"
)

// NodeChunk wraps a text chunk with its target graph node ID for UI multiplexing.
type NodeChunk struct {
	NodeID uuid.UUID
	Text   string
}

// ChatMeta separates the stable directory ID from the mutable display name.
type ChatMeta struct {
	ID                     string    `json:"id"`
	Name                   string    `json:"name"`
	CreatedAt              time.Time `json:"created_at"`
	ActiveThinkState       string    `json:"active_think_state"`
	InitializedThinkStates []string  `json:"initialized_think_states"`

	// Topology Routing
	ActiveBranch string `json:"active_branch"`

	// Memory Sliding Window State
	DigestionHorizon  uuid.UUID `json:"digestion_horizon,omitempty"`
	EvaluationHorizon uuid.UUID `json:"evaluation_horizon,omitempty"`
}

// AppConfig defines the server's global capabilities and supported domain environments.
type AppConfig struct {
	SupportedDomains []string `json:"supported_domains"`
}

// SpaceState stores the persistent environmental configuration for a physical ThinkSpace.
type SpaceState struct {
	ThinkState string `json:"think_state"`
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
