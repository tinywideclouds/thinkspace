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
