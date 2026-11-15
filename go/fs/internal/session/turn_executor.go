package session

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
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
	broadcaster      *GraphBroadcaster
}

func NewTurnExecutor(
	logger *slog.Logger,
	modelClient llm.ModelClient,
	wsService *workspace.Service,
) *TurnExecutor {
	return &TurnExecutor{
		logger:           logger,
		modelClient:      modelClient,
		workspaceService: wsService,
		broadcaster:      NewGraphBroadcasterFromService(logger, wsService),
	}
}

func inspectBundleContext(bundle cognition.ThinkBundle) (workbenchFiles []string, mapbookLayers []string, mapbookLayerBytes map[string]int, hasGolangAST bool) {
	workbenchFiles = make([]string, 0, len(bundle.Workbench))
	for _, item := range bundle.Workbench {
		workbenchFiles = append(workbenchFiles, item.Path)
	}

	mapbookLayerBytes = make(map[string]int, len(bundle.Mapbook.Maps))
	for layer, content := range bundle.Mapbook.Maps {
		mapbookLayers = append(mapbookLayers, layer)
		trimmed := strings.TrimSpace(content)
		mapbookLayerBytes[layer] = len(content)

		lowerLayer := strings.ToLower(layer)
		if trimmed != "" && (strings.Contains(lowerLayer, "ast") || strings.Contains(lowerLayer, "skeleton")) {
			hasGolangAST = true
		}
		if trimmed != "" && (strings.Contains(content, "func ") || strings.Contains(content, "type ") || strings.Contains(content, "package ")) {
			hasGolangAST = true
		}
	}
	sort.Strings(mapbookLayers)
	return workbenchFiles, mapbookLayers, mapbookLayerBytes, hasGolangAST
}

func (e *TurnExecutor) Execute(
	ctx context.Context,
	address *thinkgraph.ChatAddress,
	responseID uuid.UUID,
	parentID uuid.UUID,
	anticipateResponse bool,
	thinkState spaces.ThinkState,
	bundle cognition.ThinkBundle,
	registry *tools.Registry,
	ui tools.UI,
	emitter telemetry.Emitter,
) (signals.EngineSignal, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, thinkState.Config().TurnTimeout())
	defer cancel()

	managerModel := thinkState.Config().Roles.Manager.Model
	maxIterations := 3
	streamer := asGraphStreamer(ui)

	for iteration := 0; iteration < maxIterations; iteration++ {
		if emitter != nil {
			emitter.Emit("manager", telemetry.Event{
				ID:        uuid.NewV7(),
				Type:      "log",
				Timestamp: time.Now(),
				Message:   "Analyzing context and formulating a plan...",
			})
		}

		if anticipateResponse {
			shellEvent := thinkgraph.NewEvent(thinkgraph.EventModel, "")
			shellEvent.ID = responseID
			if address != nil && !address.GraphID.IsZero() {
				shellEvent.GraphID = address.GraphID
			}
			if parentID != uuid.Nil() {
				shellEvent.Edges = map[string]string{"parent_id": parentID.String()}
			}

			if streamer != nil {
				streamer.StreamGraphEvent(address, thinkgraph.Node{Event: shellEvent})
			}
		}

		if e.logger != nil {
			wbFiles, mbLayers, mbBytes, hasGoAST := inspectBundleContext(bundle)
			e.logger.InfoContext(ctx, "TurnExecutor dispatching LLM context bundle",
				"model", managerModel,
				"iteration", iteration,
				"workbench_file_count", len(wbFiles),
				"workbench_files", wbFiles,
				"mapbook_layers", mbLayers,
				"mapbook_layer_bytes", mbBytes,
				"has_golang_ast", hasGoAST,
				"working_memory_events", len(bundle.Trajectory.WorkingMemory),
				"long_term_memory_items", len(bundle.Trajectory.LongTermMemory),
			)
		}

		stream := e.modelClient.GenerateContentStream(
			timeoutCtx,
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
				ui.OnTextChunk(workspace.NodeChunk{NodeID: responseID, Text: chunk.Text})
				fullModelResponse.WriteString(chunk.Text)
			}
			if len(chunk.ToolCalls) > 0 {
				interceptedTools = append(interceptedTools, chunk.ToolCalls...)
			}
			if chunk.Usage != nil {
				finalUsage = chunk.Usage
			}
		}

		e.logger.InfoContext(ctx, "TurnExecutor LLM generation complete", "text_length", fullModelResponse.Len(), "tool_count", len(interceptedTools), "response_id", responseID)

		if fullModelResponse.Len() > 0 {
			var usage *telemetry.TokenUsage
			if finalUsage != nil {
				usage = &telemetry.TokenUsage{
					PromptTokens:    finalUsage.PromptTokens,
					CandidateTokens: finalUsage.CandidateTokens,
					TotalTokens:     finalUsage.TotalTokens,
				}
			}

			_ = e.workspaceService.AppendModelResponse(timeoutCtx, address, responseID, fullModelResponse.String(), usage, parentID)

			bundle.Trajectory.WorkingMemory = append(bundle.Trajectory.WorkingMemory, cognition.TrajectoryEvent{
				Role:    "model",
				Content: fullModelResponse.String(),
			})

			e.broadcaster.StreamNode(ctx, address, responseID, streamer)
		}

		if len(interceptedTools) == 0 {
			break
		}

		var yieldedSignal signals.EngineSignal
		var mergedDispatch *signals.SignalDispatchFlow

		for _, call := range interceptedTools {
			e.logger.InfoContext(ctx, "tool call intercepted", "tool", call.Name)

			tool, exists := registry.Get(call.Name)
			if !exists {
				e.logger.WarnContext(ctx, "unknown tool call intercepted", "tool", call.Name)
				continue
			}

			ec := tools.ExecutionContext{
				Address:      address,
				Bundle:       &bundle,
				UI:           ui,
				Emitter:      emitter,
				RawArguments: call.Args,
			}

			tr, err := tool.Execute(timeoutCtx, ec)
			if err != nil {
				return nil, err
			}

			if tr.Content != "" {
				bundle.Trajectory.WorkingMemory = append(bundle.Trajectory.WorkingMemory, cognition.TrajectoryEvent{
					Role:    "model",
					Content: tr.Content,
				})
			}

			if tr.Payload != nil || tr.Content != "" {
				toolNodeID := responseID
				if fullModelResponse.Len() > 0 {
					toolNodeID = uuid.NewV7()
				}

				ev := thinkgraph.NewEvent(thinkgraph.EventModel, tr.Content)
				ev.ID = toolNodeID

				if tr.Payload != nil {
					data, err := tr.Payload.GraphData()
					if err != nil {
						return nil, fmt.Errorf("failed to serialize tool payload: %w", err)
					}
					ev.Data = data
				}

				if toolNodeID != responseID {
					ev.Edges = map[string]string{"parent_id": responseID.String()}
				} else if parentID != uuid.Nil() {
					ev.Edges = map[string]string{"parent_id": parentID.String()}
				}

				if ui != nil && fullModelResponse.Len() == 0 && tr.Content != "" {
					ui.OnTextChunk(workspace.NodeChunk{NodeID: responseID, Text: tr.Content})
				}

				_ = e.broadcaster.AppendAndStream(ctx, address, ev, streamer)
			}

			if tr.Signal != nil {
				e.logger.InfoContext(ctx, "tool yielded control to supervisor", "signal_type", fmt.Sprintf("%T", tr.Signal))
				if dispatchSig, ok := tr.Signal.(*signals.SignalDispatchFlow); ok {
					if mergedDispatch == nil {
						mergedDispatch = &signals.SignalDispatchFlow{}
					}
					for _, task := range dispatchSig.Tasks {
						task.AgentID = fmt.Sprintf("agent-%d", len(mergedDispatch.Tasks)+1)
						mergedDispatch.Tasks = append(mergedDispatch.Tasks, task)
					}
					yieldedSignal = mergedDispatch
				} else if yieldedSignal == nil {
					yieldedSignal = tr.Signal
				}
			}
		}

		if yieldedSignal != nil {
			return yieldedSignal, nil
		}
	}

	return &signals.SignalTerminal{}, nil
}
