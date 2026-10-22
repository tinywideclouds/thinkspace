package session

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type Coordinator struct {
	logger           *slog.Logger
	workspaceService *workspace.Service
	modelClient      llm.ModelClient
	emitter          telemetry.Emitter
	flowEngine       *FlowEngine
	fanOutFlow       flows.Flow
	digestManager    *DigestManager
	spaceID          string
	workerPrompt     string
}

func NewCoordinator(
	logger *slog.Logger,
	workspaceService *workspace.Service,
	modelClient llm.ModelClient,
	emitter telemetry.Emitter,
	flowEngine *FlowEngine,
	fanOutFlow flows.Flow,
	digestManager *DigestManager,
	spaceID string,
	workerPrompt string,
) *Coordinator {
	return &Coordinator{
		logger:           logger,
		workspaceService: workspaceService,
		modelClient:      modelClient,
		emitter:          emitter,
		flowEngine:       flowEngine,
		fanOutFlow:       fanOutFlow,
		digestManager:    digestManager,
		spaceID:          spaceID,
		workerPrompt:     workerPrompt,
	}
}

func (c *Coordinator) RunAgentLoop(
	ctx context.Context,
	chatObj *thinkgraph.Chat,
	activeSession *ChatSession,
	thinkSpace spaces.ThinkState,
	bundle cognition.ThinkBundle,
	userInterface UserInterface,
	availableTools []llm.ToolDef,
) error {
	turnContext, cancel := context.WithTimeout(ctx, thinkSpace.Config().TurnTimeout())
	defer cancel()

	managerModel := thinkSpace.Config().Roles.Manager.Model
	maxIterations := 3

	for iteration := 0; iteration < maxIterations; iteration++ {
		c.emitter.Emit("manager", telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "log",
			Timestamp: time.Now(),
			Message:   "Manager is analyzing context and formulating a plan...",
		})

		stream := c.modelClient.GenerateContentStream(
			turnContext,
			managerModel,
			bundle,
			availableTools,
		)

		var fullModelResponse strings.Builder
		var interceptedTools []llm.ToolCall
		var finalUsage *llm.UsageMetadata

		for chunk, err := range stream {
			if err != nil {
				return fmt.Errorf("generation stream failed: %w", err)
			}

			if chunk.Text != "" {
				userInterface.OnTextChunk(chunk.Text)
				fullModelResponse.WriteString(chunk.Text)
			}

			if len(chunk.ToolCalls) > 0 {
				interceptedTools = append(interceptedTools, chunk.ToolCalls...)
			}

			if chunk.Usage != nil {
				finalUsage = chunk.Usage
			}
		}

		if fullModelResponse.Len() > 0 {
			var usage *telemetry.TokenUsage
			if finalUsage != nil {
				usage = &telemetry.TokenUsage{
					PromptTokens:    finalUsage.PromptTokens,
					CandidateTokens: finalUsage.CandidateTokens,
					TotalTokens:     finalUsage.TotalTokens,
				}
			}

			_ = c.workspaceService.LogModelResponse(turnContext, chatObj, fullModelResponse.String(), usage)
			bundle.Trajectory.WorkingMemory = append(bundle.Trajectory.WorkingMemory, cognition.TrajectoryEvent{
				Role:    "model",
				Content: fullModelResponse.String(),
			})
		}

		if len(interceptedTools) == 0 {
			c.emitter.Emit("manager", telemetry.Event{
				ID:        uuid.NewV7(),
				Type:      "log",
				Timestamp: time.Now(),
				Message:   "Manager has concluded its response.",
			})
			break
		}

		call := interceptedTools[0]
		shouldBreak, err := c.handleToolCall(turnContext, chatObj, activeSession, thinkSpace, &bundle, userInterface, call)
		if err != nil {
			return err
		}
		if shouldBreak {
			break
		}
	}

	_, err := c.workspaceService.Checkpoint(turnContext, chatObj, "End of session turn")
	return err
}

func (c *Coordinator) handleToolCall(ctx context.Context, chatObj *thinkgraph.Chat, activeSession *ChatSession, thinkSpace spaces.ThinkState, bundle *cognition.ThinkBundle, ui UserInterface, call llm.ToolCall) (bool, error) {
	switch call.Name {
	case "transition_workspace":
		return c.handleTransitionWorkspace(ctx, bundle, ui, call)
	case "query_lens":
		return c.handleQueryLens(ctx, chatObj, activeSession, bundle, ui, call)
	case "create_digest":
		return c.handleCreateDigest(ctx, chatObj, thinkSpace, bundle, call)
	case "propose_change":
		return c.handleProposeChange(ctx, chatObj, thinkSpace, bundle, ui, call)
	default:
		c.logger.WarnContext(ctx, "unknown tool call intercepted", "tool", call.Name)
		return true, nil
	}
}

func (c *Coordinator) handleTransitionWorkspace(ctx context.Context, bundle *cognition.ThinkBundle, ui UserInterface, call llm.ToolCall) (bool, error) {
	targetDomain, _ := call.Args["target_domain"].(string)
	reasoning, _ := call.Args["reasoning"].(string)

	c.logger.InfoContext(ctx, "intercepted transition request", "target", targetDomain, "reasoning", reasoning)
	c.emitter.Emit("manager", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "log",
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("Manager requested workspace transition to '%s'", targetDomain),
	})

	if ui.ConfirmTransition(targetDomain, reasoning) {
		c.emitter.Emit("manager", telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "log",
			Timestamp: time.Now(),
			Message:   "Transition approved. Rebooting workspace...",
		})
		return true, &TransitionRequest{TargetDomain: targetDomain}
	}

	c.emitter.Emit("manager", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "log",
		Timestamp: time.Now(),
		Message:   "Transition denied by user. Forcing manager to self-correct...",
	})
	ui.OnTextChunk(fmt.Sprintf("\n\n⚠️ **System Intercept:** Transition to '%s' denied.\nAsking Manager to continue in current domain...\n", targetDomain))

	bundle.Trajectory.WorkingMemory = append(bundle.Trajectory.WorkingMemory, cognition.TrajectoryEvent{
		Role:    "user",
		Content: fmt.Sprintf("[System: Transition to '%s' was denied by the user. You must continue your task within the current domain.]", targetDomain),
	})
	return false, nil
}

func (c *Coordinator) handleQueryLens(ctx context.Context, chatObj *thinkgraph.Chat, activeSession *ChatSession, bundle *cognition.ThinkBundle, ui UserInterface, call llm.ToolCall) (bool, error) {
	tag, _ := call.Args["tag"].(string)
	c.logger.InfoContext(ctx, "executing query_lens", "tag", tag)
	c.emitter.Emit("manager", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "log",
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("Manager invoked query_lens for tag: %s", tag),
	})
	ui.OnTextChunk(fmt.Sprintf("\n\n🔍 **Searching memory for #%s...**\n", tag))

	var toolResponse strings.Builder
	toolResponse.WriteString(fmt.Sprintf("[System Tool Response: query_lens(%s)]\n", tag))

	graph := c.workspaceService.Graph()
	if graph == nil {
		toolResponse.WriteString("Error: Graph database is not initialized.\n")
	} else {
		lensResult, err := graph.GetLenses(ctx, chatObj, tag)
		if err != nil {
			toolResponse.WriteString(fmt.Sprintf("Error fetching events: %v\n", err))
		} else if len(lensResult.Events) == 0 {
			toolResponse.WriteString("No historical events found for this tag.\n")
		} else {
			for _, ev := range lensResult.Events {
				toolResponse.WriteString(fmt.Sprintf("- [%s] %s: %s\n", ev.Timestamp.Format("15:04:05"), ev.Type, ev.Content))
			}
		}
	}

	bundle.Trajectory.WorkingMemory = append(bundle.Trajectory.WorkingMemory, cognition.TrajectoryEvent{
		Role:    "user",
		Content: toolResponse.String(),
	})
	return false, nil
}

func (c *Coordinator) handleCreateDigest(ctx context.Context, chatObj *thinkgraph.Chat, thinkSpace spaces.ThinkState, bundle *cognition.ThinkBundle, call llm.ToolCall) (bool, error) {
	instruction, _ := call.Args["instruction"].(string)
	topicName, _ := call.Args["topic_name"].(string)

	var tags []string
	if rawTags, ok := call.Args["tags"].([]any); ok {
		for _, rt := range rawTags {
			if tStr, ok := rt.(string); ok {
				tags = append(tags, tStr)
			}
		}
	}

	var sb strings.Builder
	for _, ev := range bundle.Trajectory.WorkingMemory {
		sb.WriteString(fmt.Sprintf("[%s]: %s\n", ev.Role, ev.Content))
	}

	err := c.digestManager.CreateManualDigest(
		ctx,
		c.workspaceService,
		chatObj,
		topicName,
		tags,
		instruction,
		sb.String(),
		thinkSpace.Config().Roles.Worker.Model,
		c.workerPrompt,
	)

	if err != nil {
		c.logger.WarnContext(ctx, "create_digest failed", "error", err)
		c.emitter.Emit("manager", telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "error",
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("Digest creation failed: %v", err),
		})
	}
	return true, nil
}

func (c *Coordinator) handleProposeChange(ctx context.Context, chatObj *thinkgraph.Chat, thinkSpace spaces.ThinkState, bundle *cognition.ThinkBundle, ui UserInterface, call llm.ToolCall) (bool, error) {
	c.emitter.Emit("manager", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "log",
		Timestamp: time.Now(),
		Message:   "Manager is proposing changes and delegating to sub-agents...",
	})

	flowContext := flows.FlowContext{
		FlowID:         fmt.Sprintf("flow-%d", time.Now().Unix()),
		SpaceID:        c.spaceID,
		BaseAgentRules: c.workerPrompt,
		FlowSummary:    "Executing requested changes.",
	}

	err := c.flowEngine.Run(ctx, chatObj, thinkSpace, c.fanOutFlow, call.Args, flowContext, ui)

	if err != nil {
		c.logger.WarnContext(ctx, "tool execution failed, bouncing back to manager", "error", err)
		c.emitter.Emit("manager", telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "error",
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("Delegation failed: %v", err),
		})
		ui.OnTextChunk(fmt.Sprintf("\n\n⚠️ **System Intercept:** %v\nAsking Manager to self-correct...\n", err))

		bundle.Trajectory.WorkingMemory = append(bundle.Trajectory.WorkingMemory, cognition.TrajectoryEvent{
			Role:    "user",
			Content: fmt.Sprintf("[System Tool Error: propose_change failed with: %v. Please review your requested target_files and parameters, and retry.]", err),
		})
		return false, nil
	}
	return true, nil
}
