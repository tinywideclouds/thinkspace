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
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

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

func (e *TurnExecutor) Execute(
	ctx context.Context,
	address *thinkgraph.ChatAddress,
	thinkState spaces.ThinkState,
	bundle cognition.ThinkBundle,
	registry *tools.Registry,
	ui tools.UI, // Narrowed from UserInterface to tools.UI
) (signals.EngineSignal, error) {
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
				return nil, fmt.Errorf("generation stream failed: %w", err)
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
		e.logger.InfoContext(ctx, "tool call intercepted", "tool", call.Name)

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

		signal, err := tool.Execute(turnContext, ec)
		if err != nil {
			return nil, err
		}

		if signal != nil {
			e.logger.InfoContext(ctx, "tool yielded control to supervisor", "signal_type", fmt.Sprintf("%T", signal))
			return signal, nil
		}
	}

	return &signals.SignalTerminal{}, nil
}
