package tools

import (
	"context"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
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

type UI interface {
	OnTextChunk(text string)
	ConfirmTransition(req signals.SignalTransition) bool
}

type Workspace interface {
	Graph() thinkgraph.GraphRepository
	WorkspaceRoot() string
}

type ExecutionContext struct {
	Address      *thinkgraph.ChatAddress
	Bundle       *cognition.ThinkBundle
	UI           UI
	RawArguments map[string]any
}

type Tool interface {
	Name() ToolName
	DefaultDescription() string
	Execute(ctx context.Context, executionContext ExecutionContext) (signals.EngineSignal, error)
}
