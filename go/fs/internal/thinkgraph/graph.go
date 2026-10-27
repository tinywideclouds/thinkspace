package thinkgraph

// Chat represents the temporal timeline of a conversation and its physical boundaries.
type Chat struct {
	ID           string
	SpaceID      string
	ActiveBranch string // e.g., "main", "candidate-123"
}

// BranchTopology maps a branch to its parent and the specific event where it diverged.
type BranchTopology struct {
	ParentBranch string
	ForkPoint    string // Event ID
}

// AttentionFocus models the cognitive focus of the immediate conversational trajectory.
type AttentionFocus struct {
	PrimaryFocus   []string
	RelatedContext []string
	DormantLenses  []string
}

// GraphData represents the backend's reconstructed memory state loaded from disk.
type GraphData struct {
	Digests  []DigestMeta
	Timeline []Event
	Focus    AttentionFocus
}
