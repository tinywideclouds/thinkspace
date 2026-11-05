package tools

import (
	"context"
	"fmt"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

const ProposeChange ToolName = "propose_change"

const (
	AgentCount    = "agent_count"
	ContextDigest = "context_digest"
	Instruction   = "instruction"
	TargetFiles   = "target_files"
)

// FanOutDispatcher defines the capability to trigger a flow execution.
type FanOutDispatcher interface {
	// The interface no longer requires UI; the dispatcher holds it natively.
	DispatchFanOut(ctx context.Context, chatObj *thinkgraph.Chat, bundle *cognition.ThinkBundle, args map[string]any, summary string) error
}

type ProposeChangeTool struct {
	dispatcher FanOutDispatcher
}

func NewProposeChangeTool(dispatcher FanOutDispatcher) *ProposeChangeTool {
	return &ProposeChangeTool{
		dispatcher: dispatcher,
	}
}

func (t *ProposeChangeTool) Name() ToolName {
	return ProposeChange
}

func (t *ProposeChangeTool) DefaultDescription() string {
	return "Delegate a task to sub-agents."
}

func (t *ProposeChangeTool) Execute(ctx context.Context, ec ExecutionContext) (bool, error) {
	agentCount := 1
	if count, ok := ec.RawArguments[AgentCount].(float64); ok { // JSON unmarshals numbers to float64
		agentCount = int(count)
	} else if count, ok := ec.RawArguments[AgentCount].(int); ok {
		agentCount = count
	}

	contextDigest, _ := ec.RawArguments[ContextDigest].(string)
	instruction, _ := ec.RawArguments[Instruction].(string)

	var targetFiles []any
	if tf, ok := ec.RawArguments[TargetFiles].([]any); ok {
		targetFiles = tf
	} else if targetFiles == nil {
		targetFiles = []any{}
	}

	if instruction == "" {
		if ec.UI != nil {
			ec.UI.OnTextChunk("\n\n⚠️ **System Intercept:** Manager provided an empty instruction. Aborting proposal.\n")
		}
		return true, fmt.Errorf("manager provided empty instruction")
	}

	if agentCount < 1 || agentCount > 5 {
		agentCount = 1
	}

	// Protect against YAML schema drift by strictly mapping flat LLM arguments
	// into the nested array structure expected by the FlowEngine.
	var tasks []any
	for i := 0; i < agentCount; i++ {
		task := map[string]any{
			"context_digest": contextDigest,
			"instruction":    instruction,
			"target_files":   targetFiles,
		}
		tasks = append(tasks, task)
	}

	flowArgs := map[string]any{
		"agent_tasks": tasks,
	}

	summary := fmt.Sprintf("Executing %d agent(s): %s", agentCount, instruction)
	if len(summary) > 80 {
		summary = summary[:77] + "..."
	}

	if ec.UI != nil {
		ec.UI.OnTextChunk(fmt.Sprintf("\n\n🚀 **Dispatching Flow:** %s\n", summary))
	}

	chatObj := &thinkgraph.Chat{ID: ec.ChatID, SpaceID: ec.SpaceID}

	// Dispatcher already has the UI stored inside it
	if err := t.dispatcher.DispatchFanOut(ctx, chatObj, ec.Bundle, flowArgs, summary); err != nil {
		return true, fmt.Errorf("dispatching fanout flow: %w", err)
	}

	return true, nil
}
