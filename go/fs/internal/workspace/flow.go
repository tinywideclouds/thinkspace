package workspace

import "context"

// FlowResult captures the outcome of an executed orchestration flow.
type FlowResult struct {
	Branches []string // e.g., ["candidate/circle-123-1", "candidate/circle-123-2"]
	Summary  string   // Aggregated context (status, diffs) for the Main Session
}

// SubAgentExecutor allows the Flow to trigger an LLM generation step without
// knowing the implementation details of the LLM provider.
// The executor must write its generated code directly into the sandboxDir.
type SubAgentExecutor func(ctx context.Context, instructions string, sandboxDir string) error

// Flow encapsulates a specific multi-step agentic lifecycle pattern.
type Flow interface {
	// Name returns the identifier that maps to the LLM Tool Call (e.g., "FanOutProposals").
	Name() string

	// Execute runs the lifecycle loop.
	Execute(ctx context.Context, svc *Service, thread *Thread, space ThinkSpace, args map[string]any, executor SubAgentExecutor) (*FlowResult, error)
}
