package flows

import "github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"

const (
	EventFlowStarted   thinkgraph.EventType = "flow_started"
	EventAgentSpawned  thinkgraph.EventType = "agent_spawned"
	EventFlowCompleted thinkgraph.EventType = "flow_completed"
)

type FlowStartedPayload struct {
	FlowID      string `json:"flow_id"`
	TaskID      string `json:"task_id"`
	FlowSummary string `json:"flow_summary"`
	AgentCount  int    `json:"agent_count"`
}

type AgentSpawnedPayload struct {
	FlowID      string `json:"flow_id"`
	AgentID     string `json:"agent_id"`
	Instruction string `json:"instruction"`
}

// FlowCompletedPayload entirely replaces the legacy workspace.FlowReceipt XML structure
type FlowCompletedPayload struct {
	FlowID  string               `json:"flow_id"`
	TaskID  string               `json:"task_id"`
	Task    string               `json:"task"`
	Summary string               `json:"summary"`
	Agents  []AgentRecordPayload `json:"agents"`
}

type AgentRecordPayload struct {
	AgentID           string `json:"agent_id"`
	Passed            bool   `json:"passed"`
	Instruction       string `json:"instruction"`
	RawPayload        string `json:"raw_payload"`
	VerificationTrace string `json:"verification_trace"`
	StateDelta        string `json:"state_delta"`
}
