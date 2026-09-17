package telemetry

import (
	"log/slog"
	"time"

	"uuid"
)

// Event is the universal structured event for all background observability.
type Event struct {
	ID        uuid.UUID
	Type      string
	Timestamp time.Time
	Message   string
	Summary   string
	Payload   map[string]any
}

// Emitter provides a decoupled Pub/Sub pipeline for orchestrators
// to stream their state back to the application layer.
type Emitter interface {
	Emit(channel string, event Event)
}

// MultiEmitter broadcasts a single event to multiple underlying emitters.
type MultiEmitter []Emitter

func (m MultiEmitter) Emit(channel string, event Event) {
	for _, e := range m {
		e.Emit(channel, event)
	}
}

// SlogEmitter translates structured telemetry events into terminal/system logs.
type SlogEmitter struct {
	logger *slog.Logger
}

func NewSlogEmitter(logger *slog.Logger) *SlogEmitter {
	return &SlogEmitter{logger: logger}
}

func (s *SlogEmitter) Emit(channel string, event Event) {
	log := s.logger.With(
		"channel", channel,
		"event_id", event.ID.String(),
		"event_type", event.Type,
	)

	for k, v := range event.Payload {
		log = log.With(k, v)
	}

	if event.Summary != "" {
		log = log.With("summary", event.Summary)
	}

	if event.Type == "error" || event.Type == "flow_error" {
		log.Warn(event.Message)
	} else {
		log.Info(event.Message)
	}
}
