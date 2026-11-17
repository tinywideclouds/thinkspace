package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
)

const TransitionThinkState ToolName = "transition_workspace"

type TransitionThinkStateRequest struct {
	TargetThinkState string `json:"target_think_state"`
	Reasoning        string `json:"reasoning"`
}

type TransitionThinkStateTool struct {
	validStates []string
}

func NewTransitionThinkStateTool(validStates []string) *TransitionThinkStateTool {
	return &TransitionThinkStateTool{validStates: validStates}
}

func (t *TransitionThinkStateTool) Name() ToolName {
	return TransitionThinkState
}

func (t *TransitionThinkStateTool) DefaultDescription() string {
	return "Transition the workspace to a different specialized think state environment."
}

func (t *TransitionThinkStateTool) ValidStates() []string {
	return t.validStates
}

func (t *TransitionThinkStateTool) Execute(ctx context.Context, executionContext ExecutionContext) (ToolResponse, error) {
	var request TransitionThinkStateRequest
	argumentBytes, err := json.Marshal(executionContext.RawArguments)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("marshaling transition arguments: %w", err)
	}
	if err := json.Unmarshal(argumentBytes, &request); err != nil {
		return ToolResponse{}, fmt.Errorf("invalid transition arguments schema: %w", err)
	}

	request.TargetThinkState = strings.TrimSpace(request.TargetThinkState)
	if request.TargetThinkState == "" {
		return ToolResponse{}, fmt.Errorf("target_think_state cannot be empty")
	}

	if len(t.validStates) > 0 && !slices.Contains(t.validStates, request.TargetThinkState) {
		if executionContext.Emitter != nil {
			executionContext.Emitter.Emit("manager", telemetry.Event{
				ID:        uuid.NewV7(),
				Type:      "system_intercept",
				Timestamp: time.Now(),
				Message:   fmt.Sprintf("Invalid transition target '%s' intercepted.", request.TargetThinkState),
			})
		}
		if executionContext.Bundle != nil {
			executionContext.Bundle.Trajectory.WorkingMemory = append(executionContext.Bundle.Trajectory.WorkingMemory, cognition.TrajectoryEvent{
				Role:    "user",
				Content: fmt.Sprintf("[System: Think state '%s' does not exist. Valid states are: %s.]", request.TargetThinkState, strings.Join(t.validStates, ", ")),
			})
		}
		return ToolResponse{}, nil
	}

	sig := signals.SignalTransition{
		TargetThinkState: request.TargetThinkState,
	}

	if executionContext.UI != nil && executionContext.UI.ConfirmTransition(request.TargetThinkState, request.Reasoning) {
		return ToolResponse{
			Signal:  &sig,
			Content: "", // Left entirely blank. Do not pollute the main timeline with a node!
			Payload: nil,
		}, nil
	}

	if executionContext.Emitter != nil {
		executionContext.Emitter.Emit("manager", telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "system_intercept",
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("Transition to '%s' denied.", request.TargetThinkState),
		})
	}

	if executionContext.Bundle != nil {
		executionContext.Bundle.Trajectory.WorkingMemory = append(executionContext.Bundle.Trajectory.WorkingMemory, cognition.TrajectoryEvent{
			Role:    "user",
			Content: fmt.Sprintf("[System: Transition to '%s' was denied by user. Continue in current state.]", request.TargetThinkState),
		})
	}

	return ToolResponse{}, nil
}
