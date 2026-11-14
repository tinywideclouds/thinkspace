package flows

import (
	"encoding/json"

	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

const (
	EventFlowLifecycle thinkgraph.EventType = "flow_lifecycle"
)

type FlowStatus int

const (
	FlowStatusUnspecified FlowStatus = 0
	FlowStatusRunning     FlowStatus = 1
	FlowStatusCompleted   FlowStatus = 2
	FlowStatusFailed      FlowStatus = 3
)

// FlowData represents the immutable parameters of a flow.
// This is serialized into the Node.Data field.
type FlowData struct {
	FlowID     string `json:"flow_id"`
	TaskID     string `json:"task_id"`
	TaskPrompt string `json:"task_prompt"`
}

// FileProposalPayload acts as the structured manifest for the Manager Node.
type FileProposalPayload struct {
	AgentCount int            `json:"agent_count"`
	Tasks      []SubAgentTask `json:"tasks"`
}

func (p FileProposalPayload) GraphData() (json.RawMessage, error) {
	return json.Marshal(p)
}

// FlowExecutionState represents the mutable, materialized state of an active flow.
// This is serialized into the Node.MaterializedState field via in-place Graph mutations.
type FlowExecutionState struct {
	Status  FlowStatus    `json:"status"`
	Summary string        `json:"summary,omitempty"`
	Error   string        `json:"error,omitempty"`
	Agents  []AgentRecord `json:"agents,omitempty"`
}

type AgentRecord struct {
	AgentID           string `json:"agent_id"`
	Passed            bool   `json:"passed"`
	Instruction       string `json:"instruction"`
	RawPayload        string `json:"raw_payload"`
	VerificationTrace string `json:"verification_trace"`
	StateDelta        string `json:"state_delta"`
}
