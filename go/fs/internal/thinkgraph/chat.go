package thinkgraph

// SubGraph represents the bounds of the currently active DAG.
type SubGraph struct {
	GraphID GraphID // The strict v7 UUID for mathematical DAG linkages (e.g., c:018e...)
	Name    string  // The human-readable semantic alias (e.g., "main", "worker-1")
}

// Chat represents the temporal timeline of a conversation and its physical boundaries.
type Chat struct {
	SubGraph
	ID      string
	SpaceID string
}

// BranchTopology maps a branch to its parent and the specific event where it diverged.
type BranchTopology struct {
	ParentBranch string
	ForkPoint    string // Event ID
}
