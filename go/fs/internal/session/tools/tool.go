package tools

import (
	"context"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

const (
	TargetThinkState = "target_think_state"
	Reasoning        = "reasoning"
)

type ToolName string

func (n ToolName) String() string {
	return string(n)
}

// UI defines the subset of UserInterface methods needed by tools.
type UI interface {
	OnTextChunk(text string)
	ConfirmTransition(targetThinkState, reasoning string) bool
}

// Workspace defines the subset of workspace operations needed by tools.
type Workspace interface {
	Graph() thinkgraph.GraphRepository
	WorkspaceRoot() string
}

type ExecutionContext struct {
	ChatID       string
	SpaceID      string
	Bundle       *cognition.ThinkBundle
	UI           UI
	RawArguments map[string]any
}

type Tool interface {
	Name() ToolName
	DefaultDescription() string
	Execute(ctx context.Context, executionContext ExecutionContext) (bool, error)
}
