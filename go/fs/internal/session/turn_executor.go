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

// TurnExecutor handles the universal LLM generation and tool interception loop.
// It relies entirely on the provided ThinkState and ChatAddress for boundaries.
type TurnExecutor struct {
	logger           *slog.Logger
	modelClient      llm.ModelClient
	workspaceService *workspace.Service
	emitter          telemetry.Emitter
}

func NewTurnExecutor(
	logger *slog.Logger,
	modelClient llm.ModelClient,
	wsService *workspace.Service,
	emitter telemetry.Emitter,
) *TurnExecutor {
	return &TurnExecutor{
		logger:           logger,
		modelClient:      modelClient,
		workspaceService: wsService,
		emitter:          emitter,
	}
}

// Execute runs a single cognitive turn bounded strictly by the provided ThinkState.
func (e *TurnExecutor) Execute(
	ctx context.Context,
	address *thinkgraph.ChatAddress,
	thinkState spaces.ThinkState,
	bundle cognition.ThinkBundle,
	registry *tools.Registry,
	ui UserInterface,
) error {
	turnContext, cancel := context.WithTimeout(ctx, thinkState.Config().TurnTimeout())
	defer cancel()

	managerModel := thinkState.Config().Roles.Manager.Model
	maxIterations := 3

	for iteration := 0; iteration < maxIterations; iteration++ {
		e.emitter.Emit("manager", telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "log",
			Timestamp: time.Now(),
			Message:   "Analyzing context and formulating a plan...",
		})

		stream := e.modelClient.GenerateContentStream(
			turnContext,
			managerModel,
			bundle,
			registry,
			thinkState.Config(),
		)

		var fullModelResponse strings.Builder
		var interceptedTools []llm.ToolCall
		var finalUsage *llm.UsageMetadata

		for chunk, err := range stream {
			if err != nil {
				return fmt.Errorf("generation stream failed: %w", err)
			}
			if chunk.Text != "" {
				ui.OnTextChunk(chunk.Text)
				fullModelResponse.WriteString(chunk.Text)
			}
			if len(chunk.ToolCalls) > 0 {
				interceptedTools = append(interceptedTools, chunk.ToolCalls...)
			}
			if chunk.Usage != nil {
				finalUsage = chunk.Usage
			}
		}

		e.logger.DebugContext(ctx, "raw LLM generation complete", "output", fullModelResponse.String())

		if fullModelResponse.Len() > 0 {
			var usage *telemetry.TokenUsage
			if finalUsage != nil {
				usage = &telemetry.TokenUsage{
					PromptTokens:    finalUsage.PromptTokens,
					CandidateTokens: finalUsage.CandidateTokens,
					TotalTokens:     finalUsage.TotalTokens,
				}
			}

			_ = e.workspaceService.LogModelResponse(turnContext, address, fullModelResponse.String(), usage)
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
		e.logger.InfoContext(ctx, "tool call intercepted", "tool", call.Name, "args", argKeys)

		tool, exists := registry.Get(call.Name)
		if !exists {
			e.logger.WarnContext(ctx, "unknown tool call intercepted", "tool", call.Name)
			break
		}

		ec := tools.ExecutionContext{
			Address:      address,
			Bundle:       &bundle,
			UI:           ui,
			RawArguments: call.Args,
		}

		shouldBreak, err := tool.Execute(turnContext, ec)

		// Catch the flow's synthesis request and rewrite the bundle to focus purely on the dossier
		if synthReq, ok := err.(*flows.SynthesisRequest); ok {
			e.logger.InfoContext(ctx, "manager synthesis requested by flow engine")

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

	_, err := e.workspaceService.Checkpoint(turnContext, address, "End of session turn")
	return err
}
