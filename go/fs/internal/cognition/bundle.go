package cognition

import "uuid"

// SystemRules defines the immutable laws of the current cognitive state.
type SystemRules struct {
	Persona               string
	FormattingConstraints string
}

// Mapbook is a literal book of maps mapping physical, conceptual, and temporal space.
type Mapbook struct {
	Maps map[string]string // e.g., "Structural", "Conceptual", "DigestRouter"
}

// TrajectoryEvent is a single beat in the working memory.
type TrajectoryEvent struct {
	ID      uuid.UUID
	Role    string // "user", "model", "system"
	Content string
}

// Trajectory represents the temporal working memory and its immediate catalyst.
type Trajectory struct {
	LongTermMemory []string          // Background context routed in by the Mapbook
	WorkingMemory  []TrajectoryEvent // The cohesive back-and-forth of recent turns
	Catalyst       TrajectoryEvent   // The triggering prompt/instruction
}

// WorkbenchItem represents a materialized focus element placed on the desk.
type WorkbenchItem struct {
	Path       string
	Content    string
	TokenCount int
}

// ThinkBundle is the finalized, structured state ready for LLM compilation or UI inspection.
type ThinkBundle struct {
	Rules      SystemRules
	Mapbook    Mapbook
	Trajectory Trajectory
	Workbench  []WorkbenchItem
	ActiveTags []string
}
