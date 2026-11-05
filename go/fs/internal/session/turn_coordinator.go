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
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type TurnCoordinator struct {
	logger           *slog.Logger
	workspaceService *workspace.Service
	modelClient      llm.ModelClient
	emitter          telemetry.Emitter
	flowEngine       *FlowEngine
	fanOutFlow       flows.Flow
	digestManager    *DigestManager
	spaceID          string
	workerPrompt     string
	workerModel      string
	registry         *tools.Registry
	thinkSpace       spaces.ThinkState
	ui               UserInterface
}

func NewTurnCoordinator(
	logger *slog.Logger,
	workspaceService *workspace.Service,
	modelClient llm.ModelClient,
	emitter telemetry.Emitter,
	flowEngine *FlowEngine,
	fanOutFlow flows.Flow,
	digestManager *DigestManager,
	spaceID string,
	workerPrompt string,
	workerModel string,
	registry *tools.Registry,
	thinkSpace spaces.ThinkState,
	ui UserInterface,
) *TurnCoordinator {
	return &TurnCoordinator{
		logger:           logger,
		workspaceService: workspaceService,
		modelClient:      modelClient,
		emitter:          emitter,
		flowEngine:       flowEngine,
		fanOutFlow:       fanOutFlow,
		digestManager:    digestManager,
		spaceID:          spaceID,
		workerPrompt:     workerPrompt,
		workerModel:      workerModel,
		registry:         registry,
		thinkSpace:       thinkSpace,
		ui:               ui,
	}
}

func (c *TurnCoordinator) RunAgentLoop(
	ctx context.Context,
	chatObj *thinkgraph.Chat,
	activeSession *ChatSession,
	thinkSpace spaces.ThinkState,
	bundle cognition.ThinkBundle,
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
			Message:   "Analyzing context and formulating a plan...",
		})

		stream := c.modelClient.GenerateContentStream(
			turnContext,
			managerModel,
			bundle,
			c.registry,
			thinkSpace.Config(),
		)

		var fullModelResponse strings.Builder
		var interceptedTools []llm.ToolCall
		var finalUsage *llm.UsageMetadata

		for chunk, err := range stream {
			if err != nil {
				return fmt.Errorf("generation stream failed: %w", err)
			}
			if chunk.Text != "" {
				c.ui.OnTextChunk(chunk.Text)
				fullModelResponse.WriteString(chunk.Text)
			}
			if len(chunk.ToolCalls) > 0 {
				interceptedTools = append(interceptedTools, chunk.ToolCalls...)
			}
			if chunk.Usage != nil {
				finalUsage = chunk.Usage
			}
		}

		c.logger.DebugContext(ctx, "raw LLM generation complete", "output", fullModelResponse.String())

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
			break
		}

		call := interceptedTools[0]

		var argKeys []string
		for k := range call.Args {
			argKeys = append(argKeys, k)
		}
		c.logger.InfoContext(ctx, "tool call intercepted", "tool", call.Name, "args", argKeys)

		tool, exists := c.registry.Get(call.Name)
		if !exists {
			c.logger.WarnContext(ctx, "unknown tool call intercepted", "tool", call.Name)
			break
		}

		ec := tools.ExecutionContext{
			ChatID:       chatObj.ID,
			SpaceID:      c.spaceID,
			Bundle:       &bundle,
			UI:           c.ui,
			RawArguments: call.Args,
		}

		shouldBreak, err := tool.Execute(turnContext, ec)

		// Catch the flow's synthesis request and rewrite the bundle to focus purely on the dossier
		if synthReq, ok := err.(*flows.SynthesisRequest); ok {
			c.logger.InfoContext(ctx, "manager synthesis requested by flow engine")

			bundle = cognition.ThinkBundle{
				Rules:     bundle.Rules,
				Mapbook:   bundle.Mapbook,
				Workbench: bundle.Workbench, // Keep pristine baseline files
				Trajectory: cognition.Trajectory{
					LongTermMemory: bundle.Trajectory.LongTermMemory, // Keep workspace background context
					Catalyst: cognition.TrajectoryEvent{
						Role:    "user",
						Content: synthReq.CompiledPrompt,
					},
					// We drop the WorkingMemory so the Manager doesn't get confused by its previous planning phase
				},
			}

			iteration = -1
			continue
		}

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

func (c *TurnCoordinator) DispatchFanOut(ctx context.Context, chatObj *thinkgraph.Chat, bundle *cognition.ThinkBundle, args map[string]any, summary string) error {
	flowContext := flows.FlowContext{
		FlowID:         uuid.NewV7().String(),
		SpaceID:        c.spaceID,
		BaseAgentRules: c.workerPrompt,
		FlowSummary:    summary,
	}

	return c.flowEngine.Run(ctx, chatObj, c.thinkSpace, c.fanOutFlow, args, flowContext, c.ui)
}

func (c *TurnCoordinator) CommitDigest(ctx context.Context, chatObj *thinkgraph.Chat, topicName string, tags []string, instruction string, contextData string) error {
	return c.digestManager.CreateManualDigest(
		ctx,
		c.workspaceService,
		chatObj,
		topicName,
		tags,
		instruction,
		contextData,
		c.workerModel,
		c.workerPrompt,
	)
}

func (c *TurnCoordinator) Graph() thinkgraph.GraphRepository {
	return c.workspaceService.Graph()
}

func (c *TurnCoordinator) WorkspaceRoot() string {
	return c.workspaceService.WorkspaceRoot()
}
