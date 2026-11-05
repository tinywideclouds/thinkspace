package session

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
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
	bundleAssembler  *BundleAssembler
	fanOutFlow       flows.Flow
	digestManager    *DigestManager
	reviewPolicy     ContextReviewPolicy
}

func NewChatSession(
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
		logger.ErrorContext(ctx, "failed initializing chat boundaries", "space_id", spaceID, "chat_id", chatID, "error", err)
		return nil, fmt.Errorf("initializing chat boundaries: %w", err)
	}

	meta, err := workspaceManager.GetChatMeta(spaceID, chatID)
	if err != nil {
		logger.ErrorContext(ctx, "failed loading chat meta", "space_id", spaceID, "chat_id", chatID, "error", err)
		return nil, fmt.Errorf("loading chat meta: %w", err)
	}

	if meta.ActiveThinkState == "" {
		if spaceState, stateErr := workspaceManager.GetSpaceState(spaceID); stateErr == nil && spaceState.ThinkState != "" {
			meta.ActiveThinkState = spaceState.ThinkState
		} else {
			meta.ActiveThinkState = "intake"
		}
		_ = workspaceManager.UpdateChatMeta(spaceID, chatID, func(m *workspace.ChatMeta) {
			m.ActiveThinkState = meta.ActiveThinkState
		})
	}

	assembler := NewBundleAssembler(
		logger,
		workspaceManager.GraphRepository(),
		thinkgraph.NewContextProjecter(),
		registry,
		workspaceService.WorkspaceRoot(),
	)

	return &ChatSession{
		SpaceID:          spaceID,
		ChatID:           chatID,
		Meta:             meta,
		logger:           logger,
		registry:         registry,
		workspaceManager: workspaceManager,
		workspaceService: workspaceService,
		modelClient:      modelClient,
		bundleAssembler:  assembler,
		fanOutFlow:       fanOutFlow,
		digestManager:    digestManager,
		reviewPolicy:     NewTokenThresholdPolicy(),
	}, nil
}

func (s *ChatSession) Mu() *sync.RWMutex {
	return &s.mu
}

func (s *ChatSession) SetReviewPolicy(policy ContextReviewPolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if policy == nil {
		policy = NewTokenThresholdPolicy()
	}
	s.reviewPolicy = policy
}

func (s *ChatSession) flushMeta() error {
	s.mu.RLock()
	activeThinkState := s.Meta.ActiveThinkState
	initializedThinkStates := s.Meta.InitializedThinkStates
	digestionHorizon := s.Meta.DigestionHorizon
	evaluationHorizon := s.Meta.EvaluationHorizon
	s.mu.RUnlock()

	return s.workspaceManager.UpdateChatMeta(s.SpaceID, s.ChatID, func(m *workspace.ChatMeta) {
		m.ActiveThinkState = activeThinkState
		m.InitializedThinkStates = initializedThinkStates
		m.DigestionHorizon = digestionHorizon
		m.EvaluationHorizon = evaluationHorizon
	})
}

func (s *ChatSession) flushMetaLocked() error {
	activeThinkState := s.Meta.ActiveThinkState
	initializedThinkStates := s.Meta.InitializedThinkStates
	digestionHorizon := s.Meta.DigestionHorizon
	evaluationHorizon := s.Meta.EvaluationHorizon

	return s.workspaceManager.UpdateChatMeta(s.SpaceID, s.ChatID, func(m *workspace.ChatMeta) {
		m.ActiveThinkState = activeThinkState
		m.InitializedThinkStates = initializedThinkStates
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
	activeThinkState := s.Meta.ActiveThinkState
	digestionHorizon := s.Meta.DigestionHorizon
	s.mu.RUnlock()

	chatObj := &thinkgraph.Chat{ID: s.ChatID, SpaceID: s.SpaceID, ActiveBranch: activeBranch}

	thinkState, ok := s.registry.GetThinkState(activeThinkState)
	if !ok {
		s.logger.WarnContext(ctx, "failed resolving think state for graph data", "thinkState", activeThinkState)
	}

	_, graphData, err := s.bundleAssembler.Assemble(ctx, chatObj, s.buildTopology(), digestionHorizon, activeThinkState, thinkState)
	return graphData, err
}

func (s *ChatSession) RunTurn(ctx context.Context, promptText string, ui UserInterface, emitter telemetry.Emitter) error {
	s.mu.RLock()
	activeBranch := s.Meta.ActiveBranch
	s.mu.RUnlock()

	chatObj := &thinkgraph.Chat{ID: s.ChatID, SpaceID: s.SpaceID, ActiveBranch: activeBranch}

	if promptText != "" {
		s.logger.InfoContext(ctx, "processing user prompt", "chat_id", s.ChatID, "prompt", promptText)
		if err := s.workspaceService.LogUserPrompt(ctx, chatObj, promptText); err != nil {
			s.logger.ErrorContext(ctx, "failed logging user prompt to ledger", "chat_id", s.ChatID, "error", err)
			return fmt.Errorf("logging user prompt: %w", err)
		}
		s.logger.InfoContext(ctx, "successfully logged user prompt to ledger", "chat_id", s.ChatID)
	}

	for {
		emitter.Emit("manager", telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "log",
			Timestamp: time.Now(),
			Message:   "Initializing workspace turn...",
		})

		s.mu.Lock()
		if s.Meta.ActiveThinkState == "" {
			spaceState, err := s.workspaceManager.GetSpaceState(s.SpaceID)
			if err != nil {
				s.logger.WarnContext(ctx, "failed reading space state during active think state resolution", "space_id", s.SpaceID, "error", err)
			}
			s.Meta.ActiveThinkState = spaceState.ThinkState
			if flushErr := s.flushMetaLocked(); flushErr != nil {
				s.logger.WarnContext(ctx, "failed flushing meta with resolved think state", "error", flushErr)
			}
		}
		activeThinkState := s.Meta.ActiveThinkState
		digestionHorizon := s.Meta.DigestionHorizon
		s.mu.Unlock()

		s.logger.InfoContext(ctx, "resolving think state from registry", "space_id", s.SpaceID, "active_think_state", activeThinkState)
		thinkState, ok := s.registry.GetThinkState(activeThinkState)
		if !ok {
			s.logger.ErrorContext(ctx, "think state configuration not found in registry", "space_id", s.SpaceID, "think_state", activeThinkState)
			return fmt.Errorf("think state configuration not found: %s", activeThinkState)
		}

		s.logger.InfoContext(ctx, "assembling context bundle", "think_state", activeThinkState, "branch", activeBranch)
		bundle, graphData, err := s.bundleAssembler.Assemble(ctx, chatObj, s.buildTopology(), digestionHorizon, activeThinkState, thinkState)
		if err != nil {
			s.logger.ErrorContext(ctx, "failed assembling bundle", "error", err)
			return fmt.Errorf("assembling bundle: %w", err)
		}

		settings, err := s.workspaceManager.GetSpaceSettings(s.SpaceID)
		if err != nil {
			s.logger.WarnContext(ctx, "failed to read space settings, using safe defaults", "error", err)
			settings = workspace.SpaceSettings{
				ReviewPolicy: workspace.ReviewPolicySettings{
					Active:         true,
					MinTokens:      200,
					IncludePersona: false,
				},
			}
		}

		s.mu.RLock()
		policy := s.reviewPolicy
		s.mu.RUnlock()
		if policy == nil {
			policy = NewTokenThresholdPolicy()
		}

		shouldReview := policy.ShouldReview(settings, &bundle, graphData.Focus)
		s.logger.InfoContext(ctx, "evaluated context review policy",
			"context_review_active", settings.ReviewPolicy.Active,
			"min_tokens", settings.ReviewPolicy.MinTokens,
			"include_persona", settings.ReviewPolicy.IncludePersona,
			"should_review", shouldReview,
			"working_memory_count", len(bundle.Trajectory.WorkingMemory),
			"workbench_files_count", len(bundle.Workbench),
		)

		if !s.contextReview(ctx, shouldReview, &bundle, graphData, ui) {
			return nil
		}

		executor := llm.NewSubAgentExecutor(s.logger, s.modelClient, thinkState.Config().Roles.Worker.Model, thinkState.Patcher())
		spaceConfig, ok := s.registry.GetConfig(activeThinkState)
		if !ok {
			s.logger.ErrorContext(ctx, "think state config not found in registry", "think_state", activeThinkState)
			return fmt.Errorf("config not found for think state: %s", activeThinkState)
		}

		registry := tools.NewRegistry()

		var validStates []string
		for k := range s.registry.GetAllConfigs() {
			validStates = append(validStates, k)
		}
		registry.Register(tools.NewTransitionThinkStateTool(validStates))

		flowEngine := NewFlowEngine(s.logger, s.workspaceService, executor, s.workspaceService.Dispatcher(), emitter)

		// The Coordinator is now injected with the UI directly.
		turnCoordinator := NewTurnCoordinator(s.logger, s.workspaceService, s.modelClient, emitter, flowEngine, s.fanOutFlow, s.digestManager, s.SpaceID, spaceConfig.WorkerSystemPrompt(), spaceConfig.Roles.Worker.Model, registry, thinkState, ui)

		registry.Register(tools.NewProposeChangeTool(turnCoordinator))
		registry.Register(tools.NewQueryLensTool(turnCoordinator))
		registry.Register(tools.NewCreateDigestTool(turnCoordinator))

		s.logger.InfoContext(ctx, "invoking turn coordinator agent loop")
		err = turnCoordinator.RunAgentLoop(ctx, chatObj, s, thinkState, bundle)

		s.evaluateAndTriggerDigestion(ctx, chatObj, thinkState.Config().Memory)

		if transitionReq, isTransition := err.(*tools.TransitionThinkStateRequest); isTransition {
			s.logger.InfoContext(ctx, "domain transition requested by LLM", "target", transitionReq.TargetThinkState)
			if rebootErr := s.handleTransition(ctx, chatObj, transitionReq.TargetThinkState, ui); rebootErr != nil {
				s.logger.ErrorContext(ctx, "domain transition failed", "error", rebootErr)
				return rebootErr
			}
			continue
		}

		if err != nil {
			s.logger.ErrorContext(ctx, "turn execution completed with error", "error", err)
		} else {
			s.logger.InfoContext(ctx, "turn execution completed successfully")
		}
		return err
	}
}

func (s *ChatSession) contextReview(ctx context.Context, shouldReview bool, bundle *cognition.ThinkBundle, graphData *thinkgraph.GraphData, ui UserInterface) bool {
	if !shouldReview {
		s.logger.DebugContext(ctx, "bypassing UI context review")
		return true
	}

	turnContext := ProjectTurnContext(s.SpaceID, s.ChatID, bundle, graphData.Focus)
	s.logger.InfoContext(ctx, "blocking for UI context review", "preview_id", turnContext.PreviewID)

	decision := ui.ReviewContext(turnContext)
	if !decision.Approved {
		s.logger.InfoContext(ctx, "turn cancelled by user during context review")
		return false
	}

	s.logger.InfoContext(ctx, "context approved by UI, applying retentions")

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
		retainedCatalyst := false
		catalystIndex := len(bundle.Trajectory.WorkingMemory)

		for _, idx := range decision.RetainedHistoryIndices {
			retainedIndices[idx] = true
			if idx == catalystIndex {
				retainedCatalyst = true
			}
		}

		if bundle.Trajectory.Catalyst.Content != "" && !retainedCatalyst {
			s.logger.WarnContext(ctx, "catalyst (user prompt) explicitly unchecked by user; aborting turn")
			return false
		}

		var prunedMemory []cognition.TrajectoryEvent
		for i, item := range bundle.Trajectory.WorkingMemory {
			if retainedIndices[i] {
				prunedMemory = append(prunedMemory, item)
			}
		}
		bundle.Trajectory.WorkingMemory = prunedMemory
	}

	s.logger.DebugContext(ctx, "final bundle prepared for orchestrator",
		"working_memory_count", len(bundle.Trajectory.WorkingMemory),
		"catalyst_present", bundle.Trajectory.Catalyst.Content != "",
		"workbench_files_count", len(bundle.Workbench),
		"long_term_memory_count", len(bundle.Trajectory.LongTermMemory),
	)

	return true
}

func (s *ChatSession) evaluateAndTriggerDigestion(ctx context.Context, chatObj *thinkgraph.Chat, memoryConfig spaces.MemoryConfig) {
	graphRepository := s.workspaceManager.GraphRepository()
	if graphRepository == nil {
		return
	}

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
	if err := s.flushMetaLocked(); err != nil {
		s.logger.WarnContext(ctx, "failed flushing meta during digestion evaluation", "error", err)
	}
	s.mu.Unlock()

	go s.executeBackgroundDigestion(chatObj, targetWindow, activeBuffer, forceDigest, targetWindowHorizonEvent.ID)
}

func (s *ChatSession) executeBackgroundDigestion(
	chatObj *thinkgraph.Chat,
	targetWindow []thinkgraph.Event,
	activeBuffer []thinkgraph.Event,
	forceDigest bool,
	targetWindowHorizonEventID uuid.UUID,
) {
	bgCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	s.mu.RLock()
	activeThinkState := s.Meta.ActiveThinkState
	s.mu.RUnlock()

	thinkState, ok := s.registry.GetThinkState(activeThinkState)
	if !ok {
		s.logger.Error("Background digestion aborted: think state not found", "think_state", activeThinkState)
		return
	}

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
		s.Meta.DigestionHorizon = targetWindowHorizonEventID
		if flushErr := s.flushMetaLocked(); flushErr != nil {
			s.logger.Warn("Failed flushing meta after digestion horizon advancement", "error", flushErr)
		}
		s.mu.Unlock()
		s.logger.Info("Digestion Horizon advanced", "new_horizon", targetWindowHorizonEventID, "digests_created", count)
	}
}

func (s *ChatSession) handleTransition(ctx context.Context, chatObj *thinkgraph.Chat, targetThinkState string, ui UserInterface) error {
	targetSpace, exists := s.registry.GetThinkState(targetThinkState)
	if !exists {
		ui.OnTextChunk(fmt.Sprintf("\n\n⚠️ **System Error:** Think state '%s' does not exist in registry. Transition aborted.\n", targetThinkState))
		return fmt.Errorf("invalid transition target: %s", targetThinkState)
	}

	chatDir := filepath.Join(s.workspaceService.WorkspaceRoot(), s.SpaceID, "chats", chatObj.ID)

	s.mu.Lock()
	hasInitialized := false
	for _, d := range s.Meta.InitializedThinkStates {
		if d == targetThinkState {
			hasInitialized = true
			break
		}
	}

	if !hasInitialized {
		if initErr := targetSpace.Initialize(ctx, chatDir); initErr != nil {
			s.mu.Unlock()
			return fmt.Errorf("failed to initialize domain %s: %w", targetThinkState, initErr)
		}
		s.Meta.InitializedThinkStates = append(s.Meta.InitializedThinkStates, targetThinkState)
	}

	s.Meta.ActiveThinkState = targetThinkState
	if err := s.flushMetaLocked(); err != nil {
		s.logger.WarnContext(ctx, "failed to flush meta after domain transition", "error", err)
	}
	s.mu.Unlock()

	if err := s.workspaceService.LogDigest(ctx, chatObj, fmt.Sprintf("Workspace transitioned to %s domain.", targetThinkState), []string{"system"}, false, nil); err != nil {
		s.logger.WarnContext(ctx, "failed logging transition digest", "error", err)
	}
	return nil
}
