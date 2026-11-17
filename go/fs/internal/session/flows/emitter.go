package flows

import "log/slog"

// MultiTelemetryEmitter broadcasts a single event to multiple underlying emitters.
type MultiTelemetryEmitter []TelemetryEmitter

func (m MultiTelemetryEmitter) Emit(channel string, event TelemetryEvent) {
	for _, e := range m {
		if e != nil {
			e.Emit(channel, event)
		}
	}
}

// SlogEmitter translates structured telemetry events into terminal/system logs.
type SlogEmitter struct {
	logger *slog.Logger
}

func NewSlogEmitter(logger *slog.Logger) *SlogEmitter {
	return &SlogEmitter{logger: logger}
}

func (s *SlogEmitter) Emit(channel string, event TelemetryEvent) {
	logger := s.logger
	if logger == nil {
		logger = slog.Default()
	}

	log := logger.With(
		"channel", channel,
		"event_id", event.ID.String(),
		"event_type", event.Type,
	)

	// Dynamically unpack the JSON payload into slog attributes
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
