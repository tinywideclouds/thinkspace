package flows

import (
	"context"

	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// BranchResult encapsulates the final state of a candidate branch produced by a flow.
type BranchResult struct {
	CandidateID string
	Passed      bool
	Trace       string
}

// FlowResult captures the outcome of an executed orchestration flow.
type FlowResult struct {
	Branches []BranchResult
	Summary  string
}

// Flow encapsulates a specific multi-step agentic lifecycle pattern.
type Flow interface {
	Name() string

	// Execute runs the lifecycle loop, streaming events via the FlowEmitter.
	Execute(
		ctx context.Context,
		service *workspace.Service,
		thread *chat.Thread,
		space spaces.ThinkSpace,
		args map[string]any,
		flowConfig FlowConfig,
		flowContext FlowContext,
		emitter FlowEmitter,
		executor workspace.SubAgentExecutor,
		verifier spaces.Verifier,
	) (*FlowResult, error)
}
