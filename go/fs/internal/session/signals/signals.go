package signals

import (
	"context"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

// GraphStreamer is the minimal UI capability needed to push graph node updates.
type GraphStreamer interface {
	StreamGraphEvent(address *thinkgraph.ChatAddress, node thinkgraph.Node)
}

// FlowHandle represents a transferable, thread-safe lifecycle controller for an active flow
// and all candidate branches spawned during its execution.
type FlowHandle interface {
	RegisterCandidate(cand flows.Candidate)
	Candidates() []flows.Candidate
	ResolveCandidate(ctx context.Context, candidateID string, accept bool, reason string) error
	Finalize(ctx context.Context, fallbackReason string) error
	SetRunning(ctx context.Context, summary string, agents []flows.AgentRecord, streamer GraphStreamer) error
	Complete(ctx context.Context, summary string, agents []flows.AgentRecord, streamer GraphStreamer) error
	Fail(ctx context.Context, summary string, errTrace string, agents []flows.AgentRecord, streamer GraphStreamer) error
}

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
	Intention string
	Tags      []string
	Tasks     []flows.SubAgentTask
}

func (s *SignalDispatchFlow) IsTerminal() bool { return false }

// SignalSynthesize indicates the FlowEngine has finished and wants the SynthesizeEngine to merge the candidate proposals.
type SignalSynthesize struct {
	OriginalInstruction string
	OriginalContext     string
	Candidates          []flows.Candidate
	Flow                FlowHandle
	FlowAddress         *thinkgraph.ChatAddress
	ParentNodeID        uuid.UUID
	AutoAccept          bool
	TargetFiles         []string
}

func (s *SignalSynthesize) IsTerminal() bool { return false }

// SignalTransition indicates the LLM wants to reboot the workspace into a new ThinkState.
// Note: Purely for routing. Display metadata is handled by ToolResponse.
type SignalTransition struct {
	TargetThinkState string
}

func (s *SignalTransition) IsTerminal() bool { return false }
