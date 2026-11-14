package signals

import (
	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

// EngineSignal represents a strictly typed intent yielded by a tool or engine back to the ChatSession supervisor.
type EngineSignal interface {
	IsTerminal() bool
}

// SignalTerminal indicates the session turn is completely finished.
type SignalTerminal struct{}

func (s *SignalTerminal) IsTerminal() bool { return true }

// SignalExecuteChat is an internal routing signal to execute generation against the active anchored Catalyst.
type SignalExecuteChat struct{}

func (s *SignalExecuteChat) IsTerminal() bool { return false }

// SignalDispatchFlow indicates the ChatEngine wants to fan-out to sub-agents.
// Note: Purely for routing. Display metadata is handled by ToolResponse.
type SignalDispatchFlow struct {
	Tasks []flows.SubAgentTask
}

func (s *SignalDispatchFlow) IsTerminal() bool { return false }

// SignalSynthesize indicates the FlowEngine has finished and wants the SynthesizeEngine to summarize the results.
type SignalSynthesize struct {
	OriginalInstruction string
	OriginalContext     string
	Candidates          []flows.BranchResult
	FlowAddress         *thinkgraph.ChatAddress
	ParentNodeID        uuid.UUID
	AutoAccept          bool
}

func (s *SignalSynthesize) IsTerminal() bool { return false }

// SignalTransition indicates the LLM wants to reboot the workspace into a new ThinkState.
// Note: Purely for routing. Display metadata is handled by ToolResponse.
type SignalTransition struct {
	TargetThinkState string
}

func (s *SignalTransition) IsTerminal() bool { return false }
