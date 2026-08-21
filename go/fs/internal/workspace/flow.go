package workspace

import "context"

// FlowResult captures the outcome of an executed orchestration flow.
type FlowResult struct {
	Branches []string // e.g., ["candidate/circle-123-1", "candidate/circle-123-2"]
	Summary  string   // Aggregated context (status, diffs) for the Main Session
}

// AgentToken wraps a text chunk with its source agent ID for UI multiplexing.
type AgentToken struct {
	AgentID int
	Text    string
}

// SubAgentExecutor allows the Flow to trigger an LLM generation step without
// knowing the implementation details of the LLM provider.
// It now accepts a channel to stream tokens back to the UI.
type SubAgentExecutor func(ctx context.Context, instructions string, sandboxDir string, agentID int, tokenChan chan<- AgentToken) error

// Flow encapsulates a specific multi-step agentic lifecycle pattern.
type Flow interface {
	// Name returns the identifier that maps to the LLM Tool Call (e.g., "FanOutProposals").
	Name() string

	// Execute runs the lifecycle loop, now accepting a multiplexing token channel.
	Execute(ctx context.Context, svc *Service, thread *Thread, space ThinkSpace, args map[string]any, executor SubAgentExecutor, tokenChan chan<- AgentToken) (*FlowResult, error)
}
