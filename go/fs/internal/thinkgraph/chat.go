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
