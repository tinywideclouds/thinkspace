package session

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"uuid"
)

type TurnManager struct {
	logger           *slog.Logger
	registry         *config.Registry
	workspaceManager *workspace.ServiceManager
	modelClient      llm.ModelClient
	playbackEngine   *chat.PlaybackEngine
	chatAssembler    *chat.Assembler
	fanOutFlow       flows.Flow
	digestFlow       flows.Flow
}

func NewTurnManager(
	logger *slog.Logger,
	registry *config.Registry,
	workspaceManager *workspace.ServiceManager,
	modelClient llm.ModelClient,
	fanOutFlow flows.Flow,
) *TurnManager {
	return &TurnManager{
		logger:           logger,
		registry:         registry,
		workspaceManager: workspaceManager,
		modelClient:      modelClient,
		playbackEngine:   chat.NewPlaybackEngine(),
		chatAssembler:    chat.NewAssembler(),
		fanOutFlow:       fanOutFlow,
		digestFlow:       flows.NewDigestFlow(logger),
	}
}

func (tm *TurnManager) LoadThreadState(ctx context.Context, spaceID, chatID string) (*chat.Thread, *chat.ChatGraph, *chat.ThreadManifest, error) {
	workspaceService := tm.workspaceManager.GetService(spaceID)
	thread, err := workspaceService.StartThread(ctx, chatID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("starting thread: %w", err)
	}

	graph, manifest, err := tm.playbackEngine.LoadState(ctx, thread)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("loading state: %w", err)
	}

	return thread, graph, manifest, nil
}

func (tm *TurnManager) ExecuteTurn(ctx context.Context, spaceID, chatID, promptText string, ui UserInterface, emitter telemetry.Emitter) error {
	emitter.Emit("manager", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "log",
		Timestamp: time.Now(),
		Message:   "Initializing workspace turn...",
	})

	spaceState, err := tm.workspaceManager.GetSpaceState(spaceID)
	if err != nil {
		return fmt.Errorf("getting space state: %w", err)
	}

	thinkSpace, ok := tm.registry.GetDomain(spaceState.Domain)
	if !ok {
		return fmt.Errorf("domain configuration not found: %s", spaceState.Domain)
	}

	spaceConfig, ok := tm.registry.GetConfig(spaceState.Domain)
	if !ok {
		return fmt.Errorf("space configuration not found: %s", spaceState.Domain)
	}

	flowConfig, ok := tm.registry.GetFlow("fanout")
	if !ok {
		return fmt.Errorf("fanout flow configuration missing")
	}

	workspaceService := tm.workspaceManager.GetService(spaceID)
	thread, err := workspaceService.StartThread(ctx, chatID)
	if err != nil {
		return fmt.Errorf("starting thread: %w", err)
	}

	var catalystEvent *chat.Event
	if promptText != "" {
		if err := workspaceService.LogUserPrompt(ctx, thread, promptText); err != nil {
			return fmt.Errorf("logging user prompt: %w", err)
		}
		catalystEvent = &chat.Event{Type: chat.EventPrompt, Content: promptText}
	}

	emitter.Emit("manager", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "log",
		Timestamp: time.Now(),
		Message:   "Assembling JIT context from workspace maps and ledger...",
	})

	graph, manifest, err := tm.playbackEngine.LoadState(ctx, thread)
	if err != nil {
		return fmt.Errorf("loading updated state: %w", err)
	}

	rawMapbook, err := thinkSpace.Mapbook().GenerateLayers(ctx, workspaceService.WorkspaceRoot())
	if err != nil {
		return fmt.Errorf("generating mapbook layers: %w", err)
	}
	stringMaps := make(map[string]string)
	for k, v := range rawMapbook {
		stringMaps[string(k)] = v
	}

	trajectory, err := tm.chatAssembler.Build(ctx, chat.AssemblyRequest{
		Manifest:     manifest,
		RecentEvents: graph.RecentEvents,
		Catalyst:     catalystEvent,
	})
	if err != nil {
		return fmt.Errorf("assembling trajectory: %w", err)
	}

	managerBundle := cognition.ThinkBundle{
		Rules: cognition.SystemRules{
			Persona: thinkSpace.Config().ManagerSystemPrompt(),
		},
		Mapbook:    cognition.Mapbook{Maps: stringMaps},
		Trajectory: trajectory,
		Workbench:  []cognition.WorkbenchItem{},
	}

	turnCtx := TurnContext{
		SpaceID:             spaceID,
		ChatID:              chatID,
		ManagerSystemPrompt: managerBundle.Rules.Persona,
		StickyDigests:       managerBundle.Trajectory.LongTermMemory,
		FileContexts:        nil,
	}

	emitter.Emit("manager", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "log",
		Timestamp: time.Now(),
		Message:   "Context assembled successfully. Yielding for frontend approval...",
	})

	if !ui.ReviewContext(turnCtx) {
		tm.logger.Info("Turn cancelled by user during context review")
		return nil
	}

	executor := llm.NewSubAgentExecutor(
		tm.logger,
		tm.modelClient,
		thinkSpace.Config().Roles.Worker.Model,
		thinkSpace.Patcher(),
	)

	delegator := NewDelegator(
		tm.logger,
		workspaceService,
		tm.modelClient,
		executor,
		tm.fanOutFlow,
		tm.digestFlow,
		emitter,
		spaceID,
		spaceConfig.WorkerSystemPrompt(),
		flowConfig,
	)

	coordinator := NewCoordinator(
		tm.logger,
		workspaceService,
		tm.modelClient,
		emitter,
		delegator,
	)

	return coordinator.ExecuteTurn(ctx, thread, manifest, thinkSpace, managerBundle, ui)
}
