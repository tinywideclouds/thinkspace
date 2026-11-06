package thinkgraph

// SubGraph represents the bounds of the currently active DAG.
type SubGraph struct {
	GraphID GraphID // The strict v7 UUID for mathematical DAG linkages (e.g., c:018e...)
	Name    string  // The human-readable semantic alias (e.g., "main", "worker-1")
}

// ChatAddress represents a specific coordinate in the workspace, combining
// its physical directory location with its cognitive DAG identity.
type ChatAddress struct {
	ProjectID string // The stable physical directory ID for the workspace and chats (e.g., "project-123")
	ChatID    string // The stable physical directory ID (e.g., "/{projectID}/chats/{chat-123}")

	SubGraph // The strict DAG coordinate (GraphID + Name)
}

// BranchTopology maps a branch to its parent and the specific event where it diverged.
type BranchTopology struct {
	ParentBranch string
	ForkPoint    string // Event ID
}
