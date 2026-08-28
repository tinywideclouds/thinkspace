package flows

import (
	"context"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// FlowResult captures the outcome of an executed orchestration flow.
type FlowResult struct {
	Branches []string // e.g., ["candidate/circle-123-1", "candidate/circle-123-2"]
	Summary  string   // Aggregated context (status, diffs) for the Main Session
}

// Flow encapsulates a specific multi-step agentic lifecycle pattern.
type Flow interface {
	// Name returns the identifier that maps to the LLM Tool Call (e.g., "FanOutFlow").
	Name() string

	// Execute runs the lifecycle loop, streaming events via the FlowEmitter.
	Execute(
		ctx context.Context,
		service *workspace.Service,
		thread *workspace.Thread,
		space workspace.ThinkSpace,
		args map[string]any,
		flowCfg FlowConfig,
		flowCtx FlowContext,
		emitter FlowEmitter,
		executor workspace.SubAgentExecutor,
		verifier workspace.Verifier,
	) (*FlowResult, error)
}
