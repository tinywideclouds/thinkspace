package session

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type TurnManager struct {
	logger           *slog.Logger
	registry         *config.Registry
	workspaceManager *workspace.ServiceManager
	modelClient      llm.ModelClient
	playbackEngine   *chat.PlaybackEngine
	chatAssembler    *chat.Assembler
	fanOutFlow       flows.Flow
	digestManager    *DigestManager
}

func NewTurnManager(
	logger *slog.Logger,
	registry *config.Registry,
	workspaceManager *workspace.ServiceManager,
	modelClient llm.ModelClient,
	fanOutFlow flows.Flow,
	digestManager *DigestManager,
) *TurnManager {
	return &TurnManager{
		logger:           logger,
		registry:         registry,
		workspaceManager: workspaceManager,
		modelClient:      modelClient,
		playbackEngine:   chat.NewPlaybackEngine(),
		chatAssembler:    chat.NewAssembler(),
		fanOutFlow:       fanOutFlow,
		digestManager:    digestManager,
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

func (tm *TurnManager) RunTurn(ctx context.Context, spaceID, chatID, promptText string, ui UserInterface, emitter telemetry.Emitter) error {
	workspaceService := tm.workspaceManager.GetService(spaceID)
	thread, err := workspaceService.StartThread(ctx, chatID)
	if err != nil {
		return fmt.Errorf("starting thread: %w", err)
	}

	if promptText != "" {
		if err := workspaceService.LogUserPrompt(ctx, thread, promptText); err != nil {
			return fmt.Errorf("logging user prompt: %w", err)
		}
	}

	for {
		emitter.Emit("manager", telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "log",
			Timestamp: time.Now(),
			Message:   "Initializing workspace turn...",
		})

		activeDomain, hasInitialized, err := tm.resolveDomain(spaceID, chatID)
		if err != nil {
			return err
		}

		thinkSpace, ok := tm.registry.GetDomain(activeDomain)
		if !ok {
			return fmt.Errorf("domain configuration not found: %s", activeDomain)
		}

		bundle, manifest, err := tm.assembleBundle(ctx, thread, workspaceService, thinkSpace, activeDomain)
		if err != nil {
			return err
		}

		turnCtx := TurnContext{
			SpaceID:             spaceID,
			ChatID:              chatID,
			ManagerSystemPrompt: bundle.Rules.Persona,
			StickyDigests:       bundle.Trajectory.LongTermMemory,
		}

		decision := ui.ReviewContext(turnCtx)
		if !decision.Approved {
			tm.logger.Info("Turn cancelled by user during context review")
			return nil
		}

		// --- PHASE 2: CONTEXT PRUNING ---
		if decision.RetainedFilePaths != nil {
			retainedPaths := make(map[string]bool)
			for _, p := range decision.RetainedFilePaths {
				retainedPaths[p] = true
			}
			var prunedWorkbench []cognition.WorkbenchItem
			for _, item := range bundle.Workbench {
				if retainedPaths[item.Path] {
					prunedWorkbench = append(prunedWorkbench, item)
				}
			}
			bundle.Workbench = prunedWorkbench
		}

		if decision.RetainedHistoryIndices != nil {
			retainedIndices := make(map[int]bool)
			for _, idx := range decision.RetainedHistoryIndices {
				retainedIndices[idx] = true
			}
			var prunedMemory []cognition.TrajectoryEvent
			for i, item := range bundle.Trajectory.WorkingMemory {
				if retainedIndices[i] {
					prunedMemory = append(prunedMemory, item)
				}
			}
			bundle.Trajectory.WorkingMemory = prunedMemory
		}

		availableTools := tm.buildToolRegistry(thinkSpace)

		executor := llm.NewSubAgentExecutor(tm.logger, tm.modelClient, thinkSpace.Config().Roles.Worker.Model, thinkSpace.Patcher())
		spaceConfig, _ := tm.registry.GetConfig(activeDomain)

		flowEngine := NewFlowEngine(tm.logger, workspaceService, executor, emitter)
		coordinator := NewCoordinator(tm.logger, workspaceService, tm.modelClient, emitter, flowEngine, tm.fanOutFlow, tm.digestManager, spaceID, spaceConfig.WorkerSystemPrompt())

		err = coordinator.RunAgentLoop(ctx, thread, manifest, thinkSpace, bundle, ui, availableTools)

		tm.evaluateAndTriggerDigestion(ctx, spaceID, thread, thinkSpace.Config().Memory)

		if transitionReq, isTransition := err.(*TransitionRequest); isTransition {
			if rebootErr := tm.handleTransition(ctx, thread, spaceID, chatID, transitionReq.TargetDomain, hasInitialized, ui); rebootErr != nil {
				return rebootErr
			}
			continue
		}

		return err
	}
}

func (tm *TurnManager) evaluateAndTriggerDigestion(ctx context.Context, spaceID string, thread *chat.Thread, memoryConfig spaces.MemoryConfig) {
	workspaceService := tm.workspaceManager.GetService(spaceID)
	chatMeta, _ := tm.workspaceManager.GetChatMeta(spaceID, thread.ID)
	graph, _, err := tm.playbackEngine.LoadState(ctx, thread)
	if err != nil || len(graph.RecentEvents) == 0 {
		return
	}

	var volatileEvents []chat.Event
	horizonIndex := -1
	evalIndex := -1

	for i, ev := range graph.RecentEvents {
		if ev.ID == chatMeta.DigestionHorizon {
			horizonIndex = i
		}
		if ev.ID == chatMeta.EvaluationHorizon {
			evalIndex = i
		}
	}

	if horizonIndex >= 0 && horizonIndex+1 < len(graph.RecentEvents) {
		volatileEvents = graph.RecentEvents[horizonIndex+1:]
	} else if horizonIndex == -1 {
		volatileEvents = graph.RecentEvents
	}

	if len(volatileEvents) == 0 {
		return
	}

	forceDigest := false
	if len(volatileEvents) >= memoryConfig.MaxVolatile {
		forceDigest = true
	} else if len(volatileEvents) >= memoryConfig.TriggerThreshold {
		evalDistance := len(graph.RecentEvents) - 1 - evalIndex
		if evalIndex != -1 && evalDistance < 4 {
			return
		}
	} else {
		return
	}

	splitIndex := len(volatileEvents) - memoryConfig.ActiveBuffer
	if splitIndex <= 0 {
		return
	}

	targetWindow := volatileEvents[:splitIndex]
	activeBuffer := volatileEvents[splitIndex:]
	newEvalHorizonEvent := graph.RecentEvents[len(graph.RecentEvents)-1]
	targetWindowHorizonEvent := targetWindow[len(targetWindow)-1]

	tm.logger.Info("Triggering background digestion", "target_events", len(targetWindow), "buffer_events", len(activeBuffer), "forced", forceDigest)

	_ = tm.workspaceManager.UpdateChatMeta(spaceID, thread.ID, func(meta *workspace.ChatMeta) {
		meta.EvaluationHorizon = newEvalHorizonEvent.ID
	})

	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		spaceState, _ := tm.workspaceManager.GetSpaceState(spaceID)
		thinkSpace, _ := tm.registry.GetDomain(spaceState.Domain)

		workerModel := thinkSpace.Config().Roles.Worker.Model
		workerPersona := thinkSpace.Config().WorkerSystemPrompt()

		count, digestErr := tm.digestManager.CreateBackgroundDigest(
			bgCtx,
			workspaceService,
			thread,
			targetWindow,
			activeBuffer,
			forceDigest,
			workerModel,
			workerPersona,
		)
		if digestErr != nil {
			tm.logger.Error("Background digestion failed", "error", digestErr)
			return
		}

		if count > 0 {
			_ = tm.workspaceManager.UpdateChatMeta(spaceID, thread.ID, func(meta *workspace.ChatMeta) {
				meta.DigestionHorizon = targetWindowHorizonEvent.ID
			})
			tm.logger.Info("Digestion Horizon advanced", "new_horizon", targetWindowHorizonEvent.ID, "digests_created", count)
		}
	}()
}

func (tm *TurnManager) resolveDomain(spaceID, chatID string) (activeDomain string, hasInitialized bool, err error) {
	chatMeta, err := tm.workspaceManager.GetChatMeta(spaceID, chatID)
	if err != nil {
		return "", false, fmt.Errorf("reading chat meta: %w", err)
	}

	activeDomain = chatMeta.ActiveDomain
	if activeDomain == "" {
		spaceState, _ := tm.workspaceManager.GetSpaceState(spaceID)
		activeDomain = spaceState.Domain
	}

	for _, d := range chatMeta.InitializedDomains {
		if d == activeDomain {
			hasInitialized = true
			break
		}
	}

	return activeDomain, hasInitialized, nil
}

func (tm *TurnManager) assembleBundle(ctx context.Context, thread *chat.Thread, service *workspace.Service, thinkSpace spaces.ThinkSpace, activeDomain string) (cognition.ThinkBundle, *chat.ThreadManifest, error) {
	graph, manifest, err := tm.playbackEngine.LoadState(ctx, thread)
	if err != nil {
		return cognition.ThinkBundle{}, nil, fmt.Errorf("loading state: %w", err)
	}

	rawMapbook, err := thinkSpace.Mapbook().GenerateLayers(ctx, service.WorkspaceRoot())
	if err != nil {
		return cognition.ThinkBundle{}, nil, fmt.Errorf("generating mapbook layers: %w", err)
	}

	stringMaps := make(map[string]string)
	for k, v := range rawMapbook {
		stringMaps[string(k)] = v
	}

	var catalystEvent *chat.Event
	if len(graph.RecentEvents) > 0 {
		last := graph.RecentEvents[len(graph.RecentEvents)-1]
		catalystEvent = &last
		graph.RecentEvents = graph.RecentEvents[:len(graph.RecentEvents)-1]
	}

	trajectory, err := tm.chatAssembler.Build(ctx, chat.AssemblyRequest{
		Manifest:     manifest,
		RecentEvents: graph.RecentEvents,
		Catalyst:     catalystEvent,
	})
	if err != nil {
		return cognition.ThinkBundle{}, nil, fmt.Errorf("assembling trajectory: %w", err)
	}

	systemPrompt := thinkSpace.Config().ManagerSystemPrompt()
	systemPrompt += fmt.Sprintf("\n\n[GLOBAL SYSTEM RULE]: You are currently operating in the '%s' domain. You may use the `transition_workspace` tool to switch to a different domain if the user's task requires different tools or a different directory structure.", activeDomain)

	bundle := cognition.ThinkBundle{
		Rules:      cognition.SystemRules{Persona: systemPrompt},
		Mapbook:    cognition.Mapbook{Maps: stringMaps},
		Trajectory: trajectory,
		Workbench:  []cognition.WorkbenchItem{},
	}

	return bundle, manifest, nil
}

func (tm *TurnManager) buildToolRegistry(thinkSpace spaces.ThinkSpace) []llm.ToolDef {
	tools := thinkSpace.Tools()
	tools = append(tools, llm.ToolDef{
		Name:        "transition_workspace",
		Description: "Transition the workspace to a different domain environment (e.g., from 'concept' to 'golang') to access new tools and physical scaffolding.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"target_domain": map[string]any{"type": "string", "description": "The name of the domain to transition to (e.g., 'golang')."},
				"reasoning":     map[string]any{"type": "string", "description": "Why this transition is necessary."},
			},
			"required": []string{"target_domain", "reasoning"},
		},
	})
	return tools
}

func (tm *TurnManager) handleTransition(ctx context.Context, thread *chat.Thread, spaceID, chatID, targetDomain string, hasInitialized bool, ui UserInterface) error {
	targetSpace, exists := tm.registry.GetDomain(targetDomain)
	if !exists {
		ui.OnTextChunk(fmt.Sprintf("\n\n⚠️ **System Error:** Domain '%s' does not exist in registry. Transition aborted.\n", targetDomain))
		return fmt.Errorf("invalid transition target: %s", targetDomain)
	}

	if !hasInitialized {
		if initErr := targetSpace.Initialize(ctx, thread.Dir); initErr != nil {
			return fmt.Errorf("failed to initialize domain %s: %w", targetDomain, initErr)
		}
	}

	tm.workspaceManager.UpdateChatMeta(spaceID, chatID, func(meta *workspace.ChatMeta) {
		meta.ActiveDomain = targetDomain
		if !hasInitialized {
			meta.InitializedDomains = append(meta.InitializedDomains, targetDomain)
		}
	})

	svc := tm.workspaceManager.GetService(spaceID)
	svc.LogDigest(ctx, thread, fmt.Sprintf("Workspace transitioned to %s domain.", targetDomain), []string{"system"}, false)
	return nil
}
