package flows

import "time"

// FlowEventType defines the generalized state machine for any agentic flow.
type FlowEventType string

const (
	FlowStart    FlowEventType = "flow_start"
	FlowSpawn    FlowEventType = "flow_spawn"
	FlowStatus   FlowEventType = "flow_status"
	FlowError    FlowEventType = "flow_error"
	FlowComplete FlowEventType = "flow_complete"
)

// FlowEvent is the universal structured event for all agentic workflows.
// It is designed to be easily serialized to JSON for WebSockets and flattened for structured logging.
type FlowEvent struct {
	FlowID    string        `json:"flow_id"`
	Type      FlowEventType `json:"type"`
	Timestamp time.Time     `json:"timestamp"`

	// Flow-Level Context
	TaskID     string `json:"task_id,omitempty"`
	AgentCount int    `json:"agent_count,omitempty"`

	// Agent-Level Context
	AgentID     string `json:"agent_id,omitempty"`
	AgentIndex  int    `json:"agent_index,omitempty"`
	Instruction string `json:"instruction,omitempty"`
	Status      string `json:"status,omitempty"` // e.g., "writing_code", "running_tests"
	Attempt     int    `json:"attempt,omitempty"`
	Trace       string `json:"trace,omitempty"` // Compiler output or test failures
	CandidateID string `json:"candidate_id,omitempty"`
	Passed      bool   `json:"passed,omitempty"`
}

// FlowEmitter provides a decoupled way for orchestrators (like FanOut)
// to stream their state back to the application layer.
type FlowEmitter interface {
	Emit(event FlowEvent)
}
