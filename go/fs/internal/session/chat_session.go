package session

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"uuid"
)

type ChatSession struct {
	mu      sync.RWMutex
	SpaceID string
	ChatID  string
	Meta    workspace.ChatMeta

	logger           *slog.Logger
	registry         *config.Registry
	workspaceManager *workspace.ServiceManager
	workspaceService *workspace.Service
	modelClient      llm.ModelClient
	contextProjecter *thinkgraph.ContextProjecter
	fanOutFlow       flows.Flow
	digestManager    *DigestManager
}

// OpenChat acts as the factory, reading physical state exactly once.
func OpenChat(
	ctx context.Context,
	logger *slog.Logger,
	registry *config.Registry,
	workspaceManager *workspace.ServiceManager,
	modelClient llm.ModelClient,
	fanOutFlow flows.Flow,
	digestManager *DigestManager,
	spaceID string,
	chatID string,
) (*ChatSession, error) {
	workspaceService := workspaceManager.GetService(spaceID)

	if _, err := workspaceService.InitializeChat(ctx, spaceID, chatID); err != nil {
		return nil, fmt.Errorf("initializing chat boundaries: %w", err)
	}

	meta, err := workspaceManager.GetChatMeta(spaceID, chatID)
	if err != nil {
		return nil, fmt.Errorf("loading chat meta: %w", err)
	}

	return &ChatSession{
		SpaceID:          spaceID,
		ChatID:           chatID,
		Meta:             meta,
		logger:           logger,
		registry:         registry,
		workspaceManager: workspaceManager,
		workspaceService: workspaceService,
		modelClient:      modelClient,
		contextProjecter: thinkgraph.NewContextProjecter(),
		fanOutFlow:       fanOutFlow,
		digestManager:    digestManager,
	}, nil
}

func (s *ChatSession) flushMeta() error {
	s.mu.RLock()
	activeDomain := s.Meta.ActiveDomain
	initializedDomains := s.Meta.InitializedDomains
	digestionHorizon := s.Meta.DigestionHorizon
	evaluationHorizon := s.Meta.EvaluationHorizon
	s.mu.RUnlock()

	return s.workspaceManager.UpdateChatMeta(s.SpaceID, s.ChatID, func(m *workspace.ChatMeta) {
		m.ActiveDomain = activeDomain
		m.InitializedDomains = initializedDomains
		m.DigestionHorizon = digestionHorizon
		m.EvaluationHorizon = evaluationHorizon
	})
}

func (s *ChatSession) buildTopology() map[string]thinkgraph.BranchTopology {
	s.mu.RLock()
	defer s.mu.RUnlock()
	topo := make(map[string]thinkgraph.BranchTopology)
	for k, v := range s.Meta.Branches {
		topo[k] = thinkgraph.BranchTopology{ParentBranch: v.ParentBranch, ForkPoint: v.ForkPoint}
	}
	return topo
}

func (s *ChatSession) GetGraphData(ctx context.Context) (*thinkgraph.GraphData, error) {
	s.mu.RLock()
	activeBranch := s.Meta.ActiveBranch
	domain := s.Meta.ActiveDomain
	s.mu.RUnlock()

	chatObj := &thinkgraph.Chat{ID: s.ChatID, SpaceID: s.SpaceID, ActiveBranch: activeBranch}

	thinkState, _ := s.registry.GetDomain(domain)
	_, graphData, err := s.assembleBundle(ctx, chatObj, thinkState)
	return graphData, err
}

func (s *ChatSession) RunTurn(ctx context.Context, promptText string, ui UserInterface, emitter telemetry.Emitter) error {
	s.mu.RLock()
	activeBranch := s.Meta.ActiveBranch
	s.mu.RUnlock()

	chatObj := &thinkgraph.Chat{ID: s.ChatID, SpaceID: s.SpaceID, ActiveBranch: activeBranch}

	if promptText != "" {
		if err := s.workspaceService.LogUserPrompt(ctx, chatObj, promptText); err != nil {
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

		s.mu.Lock()
		if s.Meta.ActiveDomain == "" {
			spaceState, _ := s.workspaceManager.GetSpaceState(s.SpaceID)
			s.Meta.ActiveDomain = spaceState.Domain
			_ = s.flushMeta()
		}
		activeDomain := s.Meta.ActiveDomain
		s.mu.Unlock()

		thinkState, ok := s.registry.GetDomain(activeDomain)
		if !ok {
			return fmt.Errorf("domain configuration not found: %s", activeDomain)
		}

		bundle, graphData, err := s.assembleBundle(ctx, chatObj, thinkState)
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
			SpaceID:             s.SpaceID,
			ChatID:              s.ChatID,
			ManagerSystemPrompt: bundle.Rules.Persona,
			StickyDigests:       bundle.Trajectory.LongTermMemory,
			RecentHistory:       recentHistory,
			FileContexts:        fileContexts,
			Focus:               graphData.Focus,
		}

		decision := ui.ReviewContext(turnCtx)
		if !decision.Approved {
			s.logger.Info("Turn cancelled by user during context review")
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

		executor := llm.NewSubAgentExecutor(s.logger, s.modelClient, thinkState.Config().Roles.Worker.Model, thinkState.Patcher())
		spaceConfig, _ := s.registry.GetConfig(activeDomain)

		// Inject the new Dispatcher dependency into the FlowEngine alongside the Emitter
		flowEngine := NewFlowEngine(s.logger, s.workspaceService, executor, s.workspaceService.Dispatcher(), emitter)
		coordinator := NewCoordinator(s.logger, s.workspaceService, s.modelClient, emitter, flowEngine, s.fanOutFlow, s.digestManager, s.SpaceID, spaceConfig.WorkerSystemPrompt())

		err = coordinator.RunAgentLoop(ctx, chatObj, s, thinkState, bundle, ui, s.buildToolRegistry(thinkState))

		s.evaluateAndTriggerDigestion(ctx, chatObj, thinkState.Config().Memory)

		if transitionReq, isTransition := err.(*TransitionRequest); isTransition {
			if rebootErr := s.handleTransition(ctx, chatObj, transitionReq.TargetDomain, ui); rebootErr != nil {
				return rebootErr
			}
			continue
		}

		return err
	}
}

func (s *ChatSession) assembleBundle(ctx context.Context, chatObj *thinkgraph.Chat, thinkState spaces.ThinkState) (cognition.ThinkBundle, *thinkgraph.GraphData, error) {
	graphRepository := s.workspaceManager.GraphRepository()
	traversalEngine := thinkgraph.NewTraversalEngine(graphRepository)
	var volatileEvents []thinkgraph.Node
	primaryTags := make(map[string]bool)
	relatedTags := make(map[string]bool)

	focusDepth := 3
	if thinkState != nil {
		focusDepth = thinkState.Config().Memory.FocusDepth
	}

	s.mu.RLock()
	digestionHorizon := s.Meta.DigestionHorizon
	s.mu.RUnlock()

	err := traversalEngine.WalkTimelineReverse(ctx, chatObj, s.buildTopology(), func(node thinkgraph.Node) bool {
		if node.ID == digestionHorizon {
			return false
		}

		volatileEvents = append(volatileEvents, node)

		if len(volatileEvents) <= focusDepth {
			for _, tag := range node.Tags {
				primaryTags[tag] = true
			}
		} else {
			for _, tag := range node.Tags {
				relatedTags[tag] = true
			}
		}

		if len(volatileEvents) >= 50 {
			return false
		}
		return true
	})
	if err != nil {
		return cognition.ThinkBundle{}, nil, fmt.Errorf("walking timeline: %w", err)
	}

	for i, j := 0, len(volatileEvents)-1; i < j; i, j = i+1, j-1 {
		volatileEvents[i], volatileEvents[j] = volatileEvents[j], volatileEvents[i]
	}

	allStickyDigests, err := graphRepository.GetStickyDigests(ctx, chatObj)
	if err != nil {
		return cognition.ThinkBundle{}, nil, fmt.Errorf("fetching digests: %w", err)
	}

	allSpaceContexts, err := graphRepository.GetSpaceContexts(ctx, chatObj)
	if err != nil {
		return cognition.ThinkBundle{}, nil, fmt.Errorf("fetching space contexts: %w", err)
	}

	manifest := thinkgraph.NewChatManifest()

	var activeDigests []thinkgraph.DigestMeta
	for _, d := range allStickyDigests {
		if len(d.Tags) == 0 {
			activeDigests = append(activeDigests, d)
			manifest.Digests[d.ID] = d
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
			manifest.Digests[d.ID] = d
		}
	}

	for _, sc := range allSpaceContexts {
		if len(sc.Tags) == 0 {
			manifest.SpaceContexts[sc.ID] = sc
			continue
		}
		isActive := false
		for _, tag := range sc.Tags {
			if primaryTags[tag] || relatedTags[tag] {
				isActive = true
				break
			}
		}
		if isActive {
			manifest.SpaceContexts[sc.ID] = sc
		}
	}

	focus := thinkgraph.AttentionFocus{
		PrimaryFocus:   mapKeys(primaryTags),
		RelatedContext: mapKeys(relatedTags),
		DormantLenses:  []string{},
	}

	graphData := &thinkgraph.GraphData{
		Timeline: volatileEvents,
		Digests:  activeDigests,
		Focus:    focus,
	}

	if thinkState == nil {
		return cognition.ThinkBundle{}, graphData, nil
	}

	rawMapbook, err := thinkState.Mapbook().GenerateLayers(ctx, s.workspaceService.WorkspaceRoot())
	if err != nil {
		return cognition.ThinkBundle{}, nil, fmt.Errorf("generating mapbook layers: %w", err)
	}

	stringMaps := make(map[string]string)
	for k, v := range rawMapbook {
		stringMaps[string(k)] = v
	}

	var catalystNode *thinkgraph.Node
	if len(volatileEvents) > 0 {
		last := volatileEvents[len(volatileEvents)-1]
		catalystNode = &last
		volatileEvents = volatileEvents[:len(volatileEvents)-1]
	}

	trajectoryRequest := thinkgraph.ContextRequest{
		Manifest:     manifest,
		RecentEvents: volatileEvents,
		Catalyst:     catalystNode,
		Focus:        focus,
	}

	// Uses the new ContextProjecter to safely ignore heavy MaterializedState diffs
	trajectory, err := s.contextProjecter.Build(ctx, trajectoryRequest)
	if err != nil {
		return cognition.ThinkBundle{}, nil, fmt.Errorf("assembling trajectory: %w", err)
	}

	systemPrompt := thinkState.Config().ManagerSystemPrompt()
	s.mu.RLock()
	domain := s.Meta.ActiveDomain
	s.mu.RUnlock()

	systemPrompt += fmt.Sprintf("\n\n[GLOBAL SYSTEM RULE]: You are currently operating in the '%s' domain. You may use the `transition_workspace` tool to switch to a different domain if the user's task requires different tools or a different directory structure.", domain)
	systemPrompt += "\n[TAXONOMY RULE]: Use precise semantic tagging. Apply existing tags from the active context to maintain a cohesive knowledge graph over time."

	bundle := cognition.ThinkBundle{
		Rules:      cognition.SystemRules{Persona: systemPrompt},
		Mapbook:    cognition.Mapbook{Maps: stringMaps},
		Trajectory: trajectory,
		Workbench:  []cognition.WorkbenchItem{},
	}

	if err := thinkState.EnrichBundle(ctx, &bundle, focus, s.workspaceService.WorkspaceRoot()); err != nil {
		return cognition.ThinkBundle{}, nil, fmt.Errorf("enriching bundle: %w", err)
	}

	return bundle, graphData, nil
}

func (s *ChatSession) buildToolRegistry(thinkState spaces.ThinkState) []llm.ToolDef {
	tools := thinkState.Tools()
	tools = append(tools, llm.ToolDef{
		Name:        "transition_workspace",
		Description: "Transition the workspace to a different domain environment.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"target_domain": map[string]any{"type": "string"},
				"reasoning":     map[string]any{"type": "string"},
			},
			"required": []string{"target_domain", "reasoning"},
		},
	})
	return tools
}

func (s *ChatSession) evaluateAndTriggerDigestion(ctx context.Context, chatObj *thinkgraph.Chat, memoryConfig spaces.MemoryConfig) {
	graphRepository := s.workspaceManager.GraphRepository()
	traversalEngine := thinkgraph.NewTraversalEngine(graphRepository)
	timeline, err := traversalEngine.GetFullTimeline(ctx, chatObj, s.buildTopology())
	if err != nil || len(timeline) == 0 {
		return
	}

	var volatileEvents []thinkgraph.Event
	horizonIndex := -1
	evalIndex := -1

	s.mu.RLock()
	digestionHorizon := s.Meta.DigestionHorizon
	evaluationHorizon := s.Meta.EvaluationHorizon
	s.mu.RUnlock()

	for i, ev := range timeline {
		if ev.ID == digestionHorizon {
			horizonIndex = i
		}
		if ev.ID == evaluationHorizon {
			evalIndex = i
		}
	}

	if horizonIndex >= 0 && horizonIndex+1 < len(timeline) {
		for _, n := range timeline[horizonIndex+1:] {
			volatileEvents = append(volatileEvents, n.Event)
		}
	} else if horizonIndex == -1 {
		for _, n := range timeline {
			volatileEvents = append(volatileEvents, n.Event)
		}
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

	s.logger.Info("Triggering background digestion", "target_events", len(targetWindow), "buffer_events", len(activeBuffer), "forced", forceDigest)

	s.mu.Lock()
	s.Meta.EvaluationHorizon = newEvalHorizonEvent.ID
	s.mu.Unlock()
	_ = s.flushMeta()

	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		s.mu.RLock()
		domain := s.Meta.ActiveDomain
		s.mu.RUnlock()

		thinkState, _ := s.registry.GetDomain(domain)
		workerModel := thinkState.Config().Roles.Worker.Model
		workerPersona := thinkState.Config().WorkerSystemPrompt()

		count, digestErr := s.digestManager.CreateBackgroundDigest(
			bgCtx,
			s.workspaceService,
			chatObj,
			targetWindow,
			activeBuffer,
			forceDigest,
			workerModel,
			workerPersona,
		)
		if digestErr != nil {
			s.logger.Error("Background digestion failed", "error", digestErr)
			return
		}

		if count > 0 {
			s.mu.Lock()
			s.Meta.DigestionHorizon = targetWindowHorizonEvent.ID
			s.mu.Unlock()
			_ = s.flushMeta()
			s.logger.Info("Digestion Horizon advanced", "new_horizon", targetWindowHorizonEvent.ID, "digests_created", count)
		}
	}()
}

func (s *ChatSession) handleTransition(ctx context.Context, chatObj *thinkgraph.Chat, targetDomain string, ui UserInterface) error {
	targetSpace, exists := s.registry.GetDomain(targetDomain)
	if !exists {
		ui.OnTextChunk(fmt.Sprintf("\n\n⚠️ **System Error:** Domain '%s' does not exist in registry. Transition aborted.\n", targetDomain))
		return fmt.Errorf("invalid transition target: %s", targetDomain)
	}

	chatDir := filepath.Join(s.workspaceService.WorkspaceRoot(), s.SpaceID, "chats", chatObj.ID)

	s.mu.Lock()
	hasInitialized := false
	for _, d := range s.Meta.InitializedDomains {
		if d == targetDomain {
			hasInitialized = true
			break
		}
	}

	if !hasInitialized {
		if initErr := targetSpace.Initialize(ctx, chatDir); initErr != nil {
			s.mu.Unlock()
			return fmt.Errorf("failed to initialize domain %s: %w", targetDomain, initErr)
		}
		s.Meta.InitializedDomains = append(s.Meta.InitializedDomains, targetDomain)
	}

	s.Meta.ActiveDomain = targetDomain
	s.mu.Unlock()
	_ = s.flushMeta()

	s.workspaceService.LogDigest(ctx, chatObj, fmt.Sprintf("Workspace transitioned to %s domain.", targetDomain), []string{"system"}, false, nil)
	return nil
}

func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
