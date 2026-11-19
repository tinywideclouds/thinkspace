package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
)

const ProposeChange ToolName = "propose_change"

const (
	AgentCount        = "agent_count"
	AgentInstructions = "agent_instructions"
	Approach          = "approach"
	AssignedTags      = "assigned_tags"
	ContextDigest     = "context_digest"
	Instruction       = "instruction"
	Intention         = "intention"
	TargetFiles       = "target_files"
)

type AgentInstruction struct {
	Approach    string `json:"approach"`
	Instruction string `json:"instruction"`
}

type ProposeChangeRequest struct {
	Intention         string             `json:"intention"`
	AssignedTags      []string           `json:"assigned_tags"`
	AgentCount        int                `json:"agent_count"`
	AgentInstructions []AgentInstruction `json:"agent_instructions"`
	ContextDigest     string             `json:"context_digest"`
	Instruction       string             `json:"instruction"`
	TargetFiles       []string           `json:"target_files"`
}

func normalizeTags(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(raw))
	var out []string
	for _, t := range raw {
		norm := strings.ToLower(strings.TrimSpace(t))
		norm = strings.TrimPrefix(norm, "#")
		if norm == "" || seen[norm] {
			continue
		}
		seen[norm] = true
		out = append(out, norm)
	}
	return out
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

func (t *ProposeChangeTool) Execute(ctx context.Context, ec ExecutionContext) (ToolResponse, error) {
	argumentBytes, err := json.Marshal(ec.RawArguments)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("marshaling propose_change arguments: %w", err)
	}

	// 1. RAW VISIBILITY: Log exactly what the LLM provided before we touch anything
	if ec.Emitter != nil {
		ec.Emitter.Emit("manager", telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "log",
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("Raw propose_change payload: %s", string(argumentBytes)),
		})
	}

	var request ProposeChangeRequest
	if err := json.Unmarshal(argumentBytes, &request); err != nil {
		return ToolResponse{}, fmt.Errorf("invalid propose_change schema: %w", err)
	}

	if strings.TrimSpace(request.Instruction) == "" && len(request.AgentInstructions) == 0 {
		if ec.Emitter != nil {
			ec.Emitter.Emit("manager", telemetry.Event{
				ID:        uuid.NewV7(),
				Type:      "system_intercept",
				Timestamp: time.Now(),
				Message:   "Manager provided an empty instruction. Aborting proposal.",
			})
		}
		return ToolResponse{}, fmt.Errorf("manager provided empty instruction")
	}

	var tasks []flows.SubAgentTask

	// 2. Map Instructions cleanly without destroying the distinct approaches
	if len(request.AgentInstructions) > 0 {
		for i, item := range request.AgentInstructions {
			body := strings.TrimSpace(item.Instruction)
			if body == "" {
				body = strings.TrimSpace(request.Instruction)
			}
			if body == "" {
				return ToolResponse{}, fmt.Errorf("agent_instructions[%d] is missing an instruction", i)
			}

			var finalInst strings.Builder
			if strings.TrimSpace(item.Approach) != "" {
				finalInst.WriteString(fmt.Sprintf("[Approach: %s]\n", strings.TrimSpace(item.Approach)))
			}
			finalInst.WriteString(body)

			tasks = append(tasks, flows.SubAgentTask{
				AgentID:       fmt.Sprintf("agent-%d", i+1),
				ContextDigest: request.ContextDigest,
				Instruction:   finalInst.String(),
				TargetFiles:   request.TargetFiles,
			})
		}
	} else {
		// Fallback: If LLM truly omitted the array, spawn identical clones based on AgentCount
		count := request.AgentCount
		if count < 1 || count > 5 {
			count = 1
		}
		for i := 0; i < count; i++ {
			tasks = append(tasks, flows.SubAgentTask{
				AgentID:       fmt.Sprintf("agent-%d", i+1),
				ContextDigest: request.ContextDigest,
				Instruction:   request.Instruction,
				TargetFiles:   request.TargetFiles,
			})
		}
	}

	intention := strings.TrimSpace(request.Intention)
	leadSummary := intention
	if leadSummary == "" {
		leadSummary = strings.TrimSpace(request.Instruction)
	}
	if leadSummary == "" && len(request.AgentInstructions) > 0 {
		leadSummary = strings.TrimSpace(request.AgentInstructions[0].Instruction)
	}

	content := fmt.Sprintf("Executing %d agent(s): %s", len(tasks), leadSummary)
	if len(content) > 160 {
		content = content[:157] + "..."
	}

	if ec.Emitter != nil {
		ec.Emitter.Emit("manager", telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "flow_dispatch",
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("Dispatching Flow: %s", content),
		})
	}

	payload := flows.FileProposalPayload{
		AgentCount: len(tasks),
		Tasks:      tasks,
	}

	return ToolResponse{
		Signal: &signals.SignalDispatchFlow{
			Intention: intention,
			Tags:      normalizeTags(request.AssignedTags),
			Tasks:     tasks,
		},
		Content: content,
		Payload: payload,
	}, nil
}
