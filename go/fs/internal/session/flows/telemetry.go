package flows

import (
	"fmt"
	"time"

	"uuid"
)

// TelemetryEvent is the universal structured event for local flow observability.
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

func (f FlowTelemetry) Summary() string {
	return fmt.Sprintf("Flow %s (%d agents)", f.FlowID, f.AgentCount)
}

// AgentTelemetry tracks micro-level execution data within the sub-agent loop.
type AgentTelemetry struct {
	FlowID      string
	AgentID     string
	AgentIndex  int
	Instruction string
	Attempt     int
	Trace       string
	CandidateID string
	Passed      bool
}

func (a AgentTelemetry) Summary() string {
	id := a.AgentID
	if id == "" {
		id = fmt.Sprintf("agent-%d", a.AgentIndex)
	}
	if a.Attempt > 0 {
		return fmt.Sprintf("Agent %s [attempt %d]", id, a.Attempt)
	}
	return fmt.Sprintf("Agent %s", id)
}

// TransitionTelemetry tracks think state domain transitions.
type TransitionTelemetry struct {
	TargetThinkState string
}

func (t TransitionTelemetry) Summary() string {
	return fmt.Sprintf("Transition to %s", t.TargetThinkState)
}
