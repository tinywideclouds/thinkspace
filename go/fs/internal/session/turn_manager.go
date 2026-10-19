package session

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"
	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type TurnManager struct {
	logger            *slog.Logger
	registry          *config.Registry
	workspaceManager  *workspace.ServiceManager
	modelClient       llm.ModelClient
	trajectoryBuilder *thinkgraph.TrajectoryBuilder
	fanOutFlow        flows.Flow
	digestManager     *DigestManager
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
		logger:            logger,
		registry:          registry,
		workspaceManager:  workspaceManager,
		modelClient:       modelClient,
		trajectoryBuilder: thinkgraph.NewTrajectoryBuilder(),
		fanOutFlow:        fanOutFlow,
		digestManager:     digestManager,
	}
}

func (tm *TurnManager) LoadChatState(ctx context.Context, spaceID, chatID string) (*thinkgraph.Chat, *thinkgraph.GraphData, *thinkgraph.ChatManifest, error) {
	workspaceService := tm.workspaceManager.GetService(spaceID)
	chatSession, err := workspaceService.InitializeChat(ctx, spaceID, chatID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("initializing chat: %w", err)
	}

	graphStore := tm.workspaceManager.GraphStore()
	if graphStore == nil {
		return nil, nil, nil, fmt.Errorf("graph store is not initialized")
	}

	_, manifest, graphData, err := tm.assembleBundle(ctx, chatSession, workspaceService, nil, "")
	if err != nil {
		return nil, nil, nil, err
	}

	return chatSession, graphData, manifest, nil
}

func (tm *TurnManager) RunTurn(ctx context.Context, spaceID, chatID, promptText string, ui UserInterface, emitter telemetry.Emitter) error {
	workspaceService := tm.workspaceManager.GetService(spaceID)
	chatSession, err := workspaceService.InitializeChat(ctx, spaceID, chatID)
	if err != nil {
		return fmt.Errorf("initializing chat: %w", err)
	}

	if promptText != "" {
		if err := workspaceService.LogUserPrompt(ctx, chatSession, promptText); err != nil {
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

		thinkState, ok := tm.registry.GetDomain(activeDomain)
		if !ok {
			return fmt.Errorf("domain configuration not found: %s", activeDomain)
		}

		bundle, manifest, graphData, err := tm.assembleBundle(ctx, chatSession, workspaceService, thinkState, activeDomain)
		if err != nil {
			return err
		}

		var recentHistory []string
		for _, mem := range bundle.Trajectory.WorkingMemory {
			recentHistory = append(recentHistory, fmt.Sprintf("[%s] %s", mem.Role, mem.Content))
		}

		var fileContexts []FileContext
		for _, item := range bundle.Workbench {
			fileContexts = append(fileContexts, FileContext{
				Path:       item.Path,
				Level:      "injected",
				TokenCount: item.TokenCount,
			})
		}

		turnCtx := TurnContext{
			SpaceID:             spaceID,
			ChatID:              chatID,
			ManagerSystemPrompt: bundle.Rules.Persona,
			StickyDigests:       bundle.Trajectory.LongTermMemory,
			RecentHistory:       recentHistory,
			FileContexts:        fileContexts,
			Focus:               graphData.Focus,
		}

		decision := ui.ReviewContext(turnCtx)
		if !decision.Approved {
			tm.logger.Info("Turn cancelled by user during context review")
			return nil
		}

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

		availableTools := tm.buildToolRegistry(thinkState)

		executor := llm.NewSubAgentExecutor(tm.logger, tm.modelClient, thinkState.Config().Roles.Worker.Model, thinkState.Patcher())
		spaceConfig, _ := tm.registry.GetConfig(activeDomain)

		flowEngine := NewFlowEngine(tm.logger, workspaceService, executor, emitter)
		coordinator := NewCoordinator(tm.logger, workspaceService, tm.modelClient, emitter, flowEngine, tm.fanOutFlow, tm.digestManager, spaceID, spaceConfig.WorkerSystemPrompt())

		err = coordinator.RunAgentLoop(ctx, chatSession, manifest, thinkState, bundle, ui, availableTools)

		tm.evaluateAndTriggerDigestion(ctx, spaceID, chatSession, thinkState.Config().Memory)

		if transitionReq, isTransition := err.(*TransitionRequest); isTransition {
			if rebootErr := tm.handleTransition(ctx, chatSession, spaceID, chatID, transitionReq.TargetDomain, hasInitialized, ui); rebootErr != nil {
				return rebootErr
			}
			continue
		}

		return err
	}
}

func (tm *TurnManager) evaluateAndTriggerDigestion(ctx context.Context, spaceID string, chatSession *thinkgraph.Chat, memoryConfig spaces.MemoryConfig) {
	chatMeta, _ := tm.workspaceManager.GetChatMeta(spaceID, chatSession.ID)
	graphStore := tm.workspaceManager.GraphStore()
	if graphStore == nil {
		return
	}

	timeline, err := graphStore.GetTimeline(ctx, chatSession)
	if err != nil || len(timeline) == 0 {
		return
	}

	var volatileEvents []thinkgraph.Event
	horizonIndex := -1
	evalIndex := -1

	for i, ev := range timeline {
		if ev.ID == chatMeta.DigestionHorizon {
			horizonIndex = i
		}
		if ev.ID == chatMeta.EvaluationHorizon {
			evalIndex = i
		}
	}

	if horizonIndex >= 0 && horizonIndex+1 < len(timeline) {
		volatileEvents = timeline[horizonIndex+1:]
	} else if horizonIndex == -1 {
		volatileEvents = timeline
	}

	if len(volatileEvents) == 0 {
		return
	}

	forceDigest := false
	if len(volatileEvents) >= memoryConfig.MaxVolatile {
		forceDigest = true
	} else if len(volatileEvents) >= memoryConfig.TriggerThreshold {
		evalDistance := len(timeline) - 1 - evalIndex
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
	newEvalHorizonEvent := timeline[len(timeline)-1]
	targetWindowHorizonEvent := targetWindow[len(targetWindow)-1]

	tm.logger.Info("Triggering background digestion", "target_events", len(targetWindow), "buffer_events", len(activeBuffer), "forced", forceDigest)

	_ = tm.workspaceManager.UpdateChatMeta(spaceID, chatSession.ID, func(meta *workspace.ChatMeta) {
		meta.EvaluationHorizon = newEvalHorizonEvent.ID
	})

	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		spaceState, _ := tm.workspaceManager.GetSpaceState(spaceID)
		thinkState, _ := tm.registry.GetDomain(spaceState.Domain)
		workspaceService := tm.workspaceManager.GetService(spaceID)

		workerModel := thinkState.Config().Roles.Worker.Model
		workerPersona := thinkState.Config().WorkerSystemPrompt()

		count, digestErr := tm.digestManager.CreateBackgroundDigest(
			bgCtx,
			workspaceService,
			chatSession,
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
			_ = tm.workspaceManager.UpdateChatMeta(spaceID, chatSession.ID, func(meta *workspace.ChatMeta) {
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

func (tm *TurnManager) assembleBundle(ctx context.Context, chatSession *thinkgraph.Chat, service *workspace.Service, thinkState spaces.ThinkState, activeDomain string) (cognition.ThinkBundle, *thinkgraph.ChatManifest, *thinkgraph.GraphData, error) {
	graphStore := tm.workspaceManager.GraphStore()
	if graphStore == nil {
		return cognition.ThinkBundle{}, nil, nil, fmt.Errorf("graph store is not initialized")
	}

	chatMeta, _ := tm.workspaceManager.GetChatMeta(chatSession.SpaceID, chatSession.ID)

	var volatileEvents []thinkgraph.Event
	primaryTags := make(map[string]bool)
	relatedTags := make(map[string]bool)

	focusDepth := 3 // Safe fallback
	if thinkState != nil {
		focusDepth = thinkState.Config().Memory.FocusDepth
	}

	err := graphStore.WalkTimelineReverse(ctx, chatSession, func(ev thinkgraph.Event) bool {
		if ev.ID == chatMeta.DigestionHorizon {
			return false // Horizon hit. Stop reading.
		}

		volatileEvents = append(volatileEvents, ev)

		if len(volatileEvents) <= focusDepth {
			for _, tag := range ev.Tags {
				primaryTags[tag] = true
			}
		} else {
			for _, tag := range ev.Tags {
				relatedTags[tag] = true
			}
		}

		if len(volatileEvents) >= 50 {
			return false
		}
		return true
	})
	if err != nil {
		return cognition.ThinkBundle{}, nil, nil, fmt.Errorf("walking timeline: %w", err)
	}

	for i, j := 0, len(volatileEvents)-1; i < j; i, j = i+1, j-1 {
		volatileEvents[i], volatileEvents[j] = volatileEvents[j], volatileEvents[i]
	}

	allStickyDigests, err := graphStore.GetStickyDigests(ctx, chatSession)
	if err != nil {
		return cognition.ThinkBundle{}, nil, nil, fmt.Errorf("fetching digests: %w", err)
	}

	var activeDigests []thinkgraph.DigestMeta
	for _, d := range allStickyDigests {
		if len(d.Tags) == 0 {
			activeDigests = append(activeDigests, d)
			continue
		}

		isActive := false
		for _, tag := range d.Tags {
			if primaryTags[tag] || relatedTags[tag] {
				isActive = true
				break
			}
		}
		if isActive {
			activeDigests = append(activeDigests, d)
		}
	}

	manifest := thinkgraph.NewChatManifest(chatSession.ID)
	for _, digest := range activeDigests {
		manifest.Digests[digest.ID] = digest
	}

	var dormantLenses []string

	focus := thinkgraph.AttentionFocus{
		PrimaryFocus:   mapKeys(primaryTags),
		RelatedContext: mapKeys(relatedTags),
		DormantLenses:  dormantLenses,
	}

	graphData := &thinkgraph.GraphData{
		Timeline: volatileEvents,
		Digests:  activeDigests,
		Focus:    focus,
	}

	if thinkState == nil {
		return cognition.ThinkBundle{}, manifest, graphData, nil
	}

	rawMapbook, err := thinkState.Mapbook().GenerateLayers(ctx, service.WorkspaceRoot())
	if err != nil {
		return cognition.ThinkBundle{}, nil, nil, fmt.Errorf("generating mapbook layers: %w", err)
	}

	stringMaps := make(map[string]string)
	for k, v := range rawMapbook {
		stringMaps[string(k)] = v
	}

	var catalystEvent *thinkgraph.Event
	if len(volatileEvents) > 0 {
		last := volatileEvents[len(volatileEvents)-1]
		catalystEvent = &last
		volatileEvents = volatileEvents[:len(volatileEvents)-1]
	}

	trajectory, err := tm.trajectoryBuilder.Build(ctx, thinkgraph.TrajectoryRequest{
		Manifest:     manifest,
		RecentEvents: volatileEvents,
		Catalyst:     catalystEvent,
		Focus:        focus,
	})
	if err != nil {
		return cognition.ThinkBundle{}, nil, nil, fmt.Errorf("assembling trajectory: %w", err)
	}

	systemPrompt := thinkState.Config().ManagerSystemPrompt()
	systemPrompt += fmt.Sprintf("\n\n[GLOBAL SYSTEM RULE]: You are currently operating in the '%s' domain. You may use the `transition_workspace` tool to switch to a different domain if the user's task requires different tools or a different directory structure.", activeDomain)

	bundle := cognition.ThinkBundle{
		Rules:      cognition.SystemRules{Persona: systemPrompt},
		Mapbook:    cognition.Mapbook{Maps: stringMaps},
		Trajectory: trajectory,
		Workbench:  []cognition.WorkbenchItem{},
	}

	if err := thinkState.EnrichBundle(ctx, &bundle, focus, service.WorkspaceRoot()); err != nil {
		return cognition.ThinkBundle{}, nil, nil, fmt.Errorf("enriching bundle: %w", err)
	}

	return bundle, manifest, graphData, nil
}

func (tm *TurnManager) buildToolRegistry(thinkState spaces.ThinkState) []llm.ToolDef {
	tools := thinkState.Tools()
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

func (tm *TurnManager) handleTransition(ctx context.Context, chatSession *thinkgraph.Chat, spaceID, chatID, targetDomain string, hasInitialized bool, ui UserInterface) error {
	targetSpace, exists := tm.registry.GetDomain(targetDomain)
	if !exists {
		ui.OnTextChunk(fmt.Sprintf("\n\n⚠️ **System Error:** Domain '%s' does not exist in registry. Transition aborted.\n", targetDomain))
		return fmt.Errorf("invalid transition target: %s", targetDomain)
	}

	svc := tm.workspaceManager.GetService(spaceID)
	chatDir := filepath.Join(svc.WorkspaceRoot(), spaceID, "chats", chatSession.ID)

	if !hasInitialized {
		if initErr := targetSpace.Initialize(ctx, chatDir); initErr != nil {
			return fmt.Errorf("failed to initialize domain %s: %w", targetDomain, initErr)
		}
	}

	tm.workspaceManager.UpdateChatMeta(spaceID, chatID, func(meta *workspace.ChatMeta) {
		meta.ActiveDomain = targetDomain
		if !hasInitialized {
			meta.InitializedDomains = append(meta.InitializedDomains, targetDomain)
		}
	})

	svc.LogDigest(ctx, chatSession, fmt.Sprintf("Workspace transitioned to %s domain.", targetDomain), []string{"system"}, false, nil)
	return nil
}

func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
