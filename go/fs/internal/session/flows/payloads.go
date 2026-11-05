package flows

import "github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"

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
