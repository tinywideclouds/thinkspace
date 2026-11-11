package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
)

const ProposeChange ToolName = "propose_change"

const (
	AgentCount    = "agent_count"
	ContextDigest = "context_digest"
	Instruction   = "instruction"
	TargetFiles   = "target_files"
)

type ProposeChangeRequest struct {
	AgentCount    int      `json:"agent_count"`
	ContextDigest string   `json:"context_digest"`
	Instruction   string   `json:"instruction"`
	TargetFiles   []string `json:"target_files"`
}

type ProposeChangeTool struct{}

func NewProposeChangeTool() *ProposeChangeTool {
	return &ProposeChangeTool{}
}

func (t *ProposeChangeTool) Name() ToolName {
	return ProposeChange
}

func (t *ProposeChangeTool) DefaultDescription() string {
	return "Delegate a task to sub-agents."
}

func (t *ProposeChangeTool) Execute(ctx context.Context, ec ExecutionContext) (signals.EngineSignal, error) {
	var request ProposeChangeRequest
	argumentBytes, err := json.Marshal(ec.RawArguments)
	if err != nil {
		return nil, fmt.Errorf("marshaling propose_change arguments: %w", err)
	}
	if err := json.Unmarshal(argumentBytes, &request); err != nil {
		return nil, fmt.Errorf("invalid propose_change schema: %w", err)
	}

	if request.Instruction == "" {
		if ec.Emitter != nil {
			ec.Emitter.Emit("manager", telemetry.Event{
				ID:        uuid.NewV7(),
				Type:      "system_intercept",
				Timestamp: time.Now(),
				Message:   "Manager provided an empty instruction. Aborting proposal.",
			})
		}
		return nil, fmt.Errorf("manager provided empty instruction")
	}

	if request.AgentCount < 1 || request.AgentCount > 5 {
		request.AgentCount = 1
	}

	var tasks []flows.SubAgentTask
	for i := 0; i < request.AgentCount; i++ {
		tasks = append(tasks, flows.SubAgentTask{
			AgentID:       fmt.Sprintf("agent-%d", i+1),
			ContextDigest: request.ContextDigest,
			Instruction:   request.Instruction,
			TargetFiles:   request.TargetFiles,
		})
	}

	summary := fmt.Sprintf("Executing %d agent(s): %s", request.AgentCount, request.Instruction)
	if len(summary) > 80 {
		summary = summary[:77] + "..."
	}

	if ec.Emitter != nil {
		ec.Emitter.Emit("manager", telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "flow_dispatch",
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("Dispatching Flow: %s", summary),
		})
	}

	return &signals.SignalDispatchFlow{
		Tasks:   tasks,
		Summary: summary,
	}, nil
}
