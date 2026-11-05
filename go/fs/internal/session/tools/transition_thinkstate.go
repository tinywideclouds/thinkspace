package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
)

const (
	TransitionThinkState ToolName = "transition_workspace"
)

type TransitionThinkStateRequest struct {
	TargetThinkState string `json:"target_think_state"`
	Reasoning        string `json:"reasoning"`
}

func (r *TransitionThinkStateRequest) Error() string {
	return "think state transition requested to: " + r.TargetThinkState
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

func (t *TransitionThinkStateTool) Execute(ctx context.Context, executionContext ExecutionContext) (bool, error) {
	var request TransitionThinkStateRequest
	argumentBytes, err := json.Marshal(executionContext.RawArguments)
	if err != nil {
		return false, fmt.Errorf("marshaling transition arguments: %w", err)
	}
	if err := json.Unmarshal(argumentBytes, &request); err != nil {
		return false, fmt.Errorf("invalid transition arguments schema: %w", err)
	}

	if request.TargetThinkState == "" {
		return false, fmt.Errorf("%s cannot be empty", TargetThinkState)
	}

	if executionContext.UI.ConfirmTransition(request.TargetThinkState, request.Reasoning) {
		return true, &request
	}

	executionContext.UI.OnTextChunk(fmt.Sprintf("\n\n⚠️ **System Intercept:** Transition to '%s' denied.\n", request.TargetThinkState))
	executionContext.Bundle.Trajectory.WorkingMemory = append(executionContext.Bundle.Trajectory.WorkingMemory, cognition.TrajectoryEvent{
		Role:    "user",
		Content: fmt.Sprintf("[System: Transition to '%s' was denied by user. Continue in current state.]", request.TargetThinkState),
	})

	return false, nil
}