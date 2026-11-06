package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
)

const TransitionThinkState ToolName = "transition_workspace"

// TransitionThinkStateRequest acts as the local JSON schema for parsing LLM arguments
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

func (t *TransitionThinkStateTool) Execute(ctx context.Context, executionContext ExecutionContext) (signals.EngineSignal, error) {
	var request TransitionThinkStateRequest
	argumentBytes, err := json.Marshal(executionContext.RawArguments)
	if err != nil {
		return nil, fmt.Errorf("marshaling transition arguments: %w", err)
	}
	if err := json.Unmarshal(argumentBytes, &request); err != nil {
		return nil, fmt.Errorf("invalid transition arguments schema: %w", err)
	}

	if request.TargetThinkState == "" {
		return nil, fmt.Errorf("target_think_state cannot be empty")
	}

	sig := signals.SignalTransition{
		TargetThinkState: request.TargetThinkState,
		Reasoning:        request.Reasoning,
	}

	if executionContext.UI.ConfirmTransition(sig) {
		return &sig, nil
	}

	executionContext.UI.OnTextChunk(fmt.Sprintf("\n\n⚠️ **System Intercept:** Transition to '%s' denied.\n", request.TargetThinkState))
	executionContext.Bundle.Trajectory.WorkingMemory = append(executionContext.Bundle.Trajectory.WorkingMemory, cognition.TrajectoryEvent{
		Role:    "user",
		Content: fmt.Sprintf("[System: Transition to '%s' was denied by user. Continue in current state.]", request.TargetThinkState),
	})

	return nil, nil
}
