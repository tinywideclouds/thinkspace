package flows

import "log/slog"

// MultiFlowEmitter broadcasts a single event to multiple underlying emitters.
type MultiFlowEmitter []FlowEmitter

func (m MultiFlowEmitter) Emit(event FlowEvent) {
	for _, e := range m {
		e.Emit(event)
	}
}

// SlogEmitter translates structured flow events into terminal/system logs.
type SlogEmitter struct {
	logger *slog.Logger
}

func NewSlogEmitter(logger *slog.Logger) *SlogEmitter {
	return &SlogEmitter{logger: logger}
}

func (s *SlogEmitter) Emit(event FlowEvent) {
	log := s.logger.With(
		"flow_id", event.FlowID,
		"event_type", event.Type,
	)

	if event.AgentID != "" {
		log = log.With("agent_id", event.AgentID, "attempt", event.Attempt)
	}

	switch event.Type {
	case FlowStart:
		log.Info("flow started", "task_id", event.TaskID, "agent_count", event.AgentCount)
	case FlowSpawn:
		log.Info("agent spawned", "instruction", event.Instruction)
	case FlowStatus:
		log.Info("status update", "status", event.Status)
	case FlowError:
		log.Warn("verification failed", "trace", event.Trace)
	case FlowComplete:
		if event.Passed {
			log.Info("flow completed successfully", "candidate_id", event.CandidateID)
		} else {
			log.Warn("flow exhausted retries", "candidate_id", event.CandidateID, "final_trace", event.Trace)
		}
	}
}
