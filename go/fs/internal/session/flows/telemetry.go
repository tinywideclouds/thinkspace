package flows

import (
	"time"

	"uuid"
)

// TelemetryEvent is the universal structured event for all background observability.
type TelemetryEvent struct {
	ID        uuid.UUID
	Type      string
	Timestamp time.Time
	Message   string
	Summary   string
	Payload   map[string]any
}

// TelemetryEmitter provides a decoupled Pub/Sub pipeline for orchestrators
// to stream their state back to the application layer based on a named channel.
type TelemetryEmitter interface {
	Emit(channel string, event TelemetryEvent)
}

// FlowTelemetry tracks macro-level execution data when a flow is instantiated.
type FlowTelemetry struct {
	FlowID     string
	TaskID     string
	AgentCount int
}

func (f FlowTelemetry) ToMap() map[string]any {
	m := make(map[string]any)
	if f.FlowID != "" {
		m["flow_id"] = f.FlowID
	}
	if f.TaskID != "" {
		m["task_id"] = f.TaskID
	}
	if f.AgentCount != 0 {
		m["agent_count"] = f.AgentCount
	}
	return m
}

// AgentTelemetry tracks micro-level execution data within the sub-agent loop.
type AgentTelemetry struct {
	FlowID      string
	AgentIndex  int
	Instruction string
	Attempt     int
	Trace       string
	CandidateID string
	Passed      bool
}

func (a AgentTelemetry) ToMap() map[string]any {
	m := make(map[string]any)
	if a.FlowID != "" {
		m["flow_id"] = a.FlowID
	}
	if a.AgentIndex != 0 {
		m["agent_index"] = a.AgentIndex
	}
	if a.Instruction != "" {
		m["instruction"] = a.Instruction
	}
	if a.Attempt != 0 {
		m["attempt"] = a.Attempt
	}
	if a.Trace != "" {
		m["trace"] = a.Trace
	}
	if a.CandidateID != "" {
		m["candidate_id"] = a.CandidateID
	}
	m["passed"] = a.Passed
	return m
}
