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
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// ChatEngine orchestrates the open-ended conversational trunk (the main DAG).
type ChatEngine struct {
	logger           *slog.Logger
	session          *ChatSession
	registry         *config.Registry
	workspaceManager *workspace.ServiceManager
	workspaceService *workspace.Service
	bundleAssembler  *BundleAssembler
	digestManager    *DigestManager
	turnExecutor     *TurnExecutor
	fanOutFlow       flows.Flow
}

func NewChatEngine(
	logger *slog.Logger,
	session *ChatSession,
	registry *config.Registry,
	workspaceManager *workspace.ServiceManager,
	workspaceService *workspace.Service,
	bundleAssembler *BundleAssembler,
	digestManager *DigestManager,
	turnExecutor *TurnExecutor,
	fanOutFlow flows.Flow,
) *ChatEngine {
	return &ChatEngine{
		logger:           logger,
		session:          session,
		registry:         registry,
		workspaceManager: workspaceManager,
		workspaceService: workspaceService,
		bundleAssembler:  bundleAssembler,
		digestManager:    digestManager,
		turnExecutor:     turnExecutor,
		fanOutFlow:       fanOutFlow,
	}
}

func (e *ChatEngine) Run(ctx context.Context, promptText string, ui UserInterface, emitter telemetry.Emitter) error {
	e.session.mu.RLock()
	activeBranch := e.session.Meta.ActiveBranch
	e.session.mu.RUnlock()

	address := &thinkgraph.ChatAddress{
		ChatID:    e.session.ChatID,
		ProjectID: e.session.SpaceID,
		SubGraph: thinkgraph.SubGraph{
			Name: activeBranch,
		},
	}

	e.turnExecutor.emitter = emitter

	if promptText != "" {
		e.logger.InfoContext(ctx, "processing user prompt", "chat_id", e.session.ChatID, "prompt", promptText)
		if err := e.workspaceService.LogUserPrompt(ctx, address, promptText); err != nil {
			e.logger.ErrorContext(ctx, "failed logging user prompt to ledger", "chat_id", e.session.ChatID, "error", err)
			return fmt.Errorf("logging user prompt: %w", err)
		}
		e.logger.InfoContext(ctx, "successfully logged user prompt to ledger", "chat_id", e.session.ChatID)
	}

	for {
		emitter.Emit("manager", telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "log",
			Timestamp: time.Now(),
			Message:   "Initializing workspace turn...",
		})

		e.session.mu.Lock()
		if e.session.Meta.ActiveThinkState == "" {
			spaceState, err := e.workspaceManager.GetSpaceState(e.session.SpaceID)
			if err != nil {
				e.logger.WarnContext(ctx, "failed reading space state during active think state resolution", "space_id", e.session.SpaceID, "error", err)
			}
			e.session.Meta.ActiveThinkState = spaceState.ThinkState
			if flushErr := e.session.flushMetaLocked(); flushErr != nil {
				e.logger.WarnContext(ctx, "failed flushing meta with resolved think state", "error", flushErr)
			}
		}
		activeThinkState := e.session.Meta.ActiveThinkState
		digestionHorizon := e.session.Meta.DigestionHorizon
		e.session.mu.Unlock()

		e.logger.InfoContext(ctx, "resolving think state from registry", "space_id", e.session.SpaceID, "active_think_state", activeThinkState)
		thinkState, ok := e.registry.GetThinkState(activeThinkState)
		if !ok {
			e.logger.ErrorContext(ctx, "think state configuration not found in registry", "space_id", e.session.SpaceID, "think_state", activeThinkState)
			return fmt.Errorf("think state configuration not found: %s", activeThinkState)
		}

		e.logger.InfoContext(ctx, "assembling context bundle", "think_state", activeThinkState, "branch", activeBranch)
		bundle, graphData, err := e.bundleAssembler.Assemble(ctx, address, e.session.buildTopology(), digestionHorizon, activeThinkState, thinkState)
		if err != nil {
			e.logger.ErrorContext(ctx, "failed assembling bundle", "error", err)
			return fmt.Errorf("assembling bundle: %w", err)
		}

		settings, err := e.workspaceManager.GetSpaceSettings(e.session.SpaceID)
		if err != nil {
			e.logger.WarnContext(ctx, "failed to read space settings, using safe defaults", "error", err)
			settings = workspace.SpaceSettings{
				ReviewPolicy: workspace.ReviewPolicySettings{
					Active:         true,
					MinTokens:      200,
					IncludePersona: false,
				},
			}
		}

		e.session.mu.RLock()
		policy := e.session.reviewPolicy
		e.session.mu.RUnlock()
		if policy == nil {
			policy = NewTokenThresholdPolicy()
		}

		shouldReview := policy.ShouldReview(settings, &bundle, graphData.Focus)
		e.logger.InfoContext(ctx, "evaluated context review policy",
			"context_review_active", settings.ReviewPolicy.Active,
			"min_tokens", settings.ReviewPolicy.MinTokens,
			"include_persona", settings.ReviewPolicy.IncludePersona,
			"should_review", shouldReview,
			"working_memory_count", len(bundle.Trajectory.WorkingMemory),
			"workbench_files_count", len(bundle.Workbench),
		)

		if !e.contextReview(ctx, shouldReview, &bundle, graphData, ui) {
			return nil
		}

		flowEngine := NewFlowEngine(e.logger, e.workspaceService, e.turnExecutor, e.workspaceService.Dispatcher(), emitter)

		registry := tools.NewRegistry()

		var validStates []string
		for k := range e.registry.GetAllConfigs() {
			validStates = append(validStates, k)
		}
		registry.Register(tools.NewTransitionThinkStateTool(validStates))
		registry.Register(tools.NewQueryLensTool(e.workspaceService))

		fanOutAdapter := NewFanOutAdapter(flowEngine, thinkState, e.session.SpaceID, ui, e.fanOutFlow)
		registry.Register(tools.NewProposeChangeTool(fanOutAdapter))

		digestAdapter := NewDigestAdapter(e.digestManager, e.workspaceService, thinkState.Config().Roles.Worker.Model, thinkState.Config().WorkerSystemPrompt())
		registry.Register(tools.NewCreateDigestTool(digestAdapter))

		e.logger.InfoContext(ctx, "invoking turn executor agent loop")
		err = e.turnExecutor.Execute(ctx, address, thinkState, bundle, registry, ui)

		e.evaluateAndTriggerDigestion(ctx, address, thinkState.Config().Memory)

		if transitionReq, isTransition := err.(*tools.TransitionThinkStateRequest); isTransition {
			e.logger.InfoContext(ctx, "domain transition requested by LLM", "target", transitionReq.TargetThinkState)
			if rebootErr := e.handleTransition(ctx, address, transitionReq.TargetThinkState, ui); rebootErr != nil {
				e.logger.ErrorContext(ctx, "domain transition failed", "error", rebootErr)
				return rebootErr
			}
			continue
		}

		if err != nil {
			e.logger.ErrorContext(ctx, "turn execution completed with error", "error", err)
		} else {
			e.logger.InfoContext(ctx, "turn execution completed successfully")
		}
		return err
	}
}

func (e *ChatEngine) contextReview(ctx context.Context, shouldReview bool, bundle *cognition.ThinkBundle, graphData *thinkgraph.GraphData, ui UserInterface) bool {
	if !shouldReview {
		e.logger.DebugContext(ctx, "bypassing UI context review")
		return true
	}

	turnContext := ProjectTurnContext(e.session.SpaceID, e.session.ChatID, bundle, graphData.Focus)
	e.logger.InfoContext(ctx, "blocking for UI context review", "preview_id", turnContext.PreviewID)

	decision := ui.ReviewContext(turnContext)
	if !decision.Approved {
		e.logger.InfoContext(ctx, "turn cancelled by user during context review")
		return false
	}

	e.logger.InfoContext(ctx, "context approved by UI, applying retentions")

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
			e.logger.WarnContext(ctx, "catalyst (user prompt) explicitly unchecked by user; aborting turn")
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

	e.logger.DebugContext(ctx, "final bundle prepared for orchestrator",
		"working_memory_count", len(bundle.Trajectory.WorkingMemory),
		"catalyst_present", bundle.Trajectory.Catalyst.Content != "",
		"workbench_files_count", len(bundle.Workbench),
		"long_term_memory_count", len(bundle.Trajectory.LongTermMemory),
	)

	return true
}

func (e *ChatEngine) evaluateAndTriggerDigestion(ctx context.Context, address *thinkgraph.ChatAddress, memoryConfig spaces.MemoryConfig) {
	graphRepository := e.workspaceManager.GraphRepository()
	if graphRepository == nil {
		return
	}

	traversalEngine := thinkgraph.NewTraversalEngine(graphRepository)
	timeline, err := traversalEngine.GetFullTimeline(ctx, address, e.session.buildTopology())
	if err != nil || len(timeline) == 0 {
		return
	}

	var volatileEvents []thinkgraph.Event
	horizonIndex := -1
	evalIndex := -1

	e.session.mu.RLock()
	digestionHorizon := e.session.Meta.DigestionHorizon
	evaluationHorizon := e.session.Meta.EvaluationHorizon
	e.session.mu.RUnlock()

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

	e.logger.Info("Triggering background digestion", "target_events", len(targetWindow), "buffer_events", len(activeBuffer), "forced", forceDigest)

	e.session.mu.Lock()
	e.session.Meta.EvaluationHorizon = newEvalHorizonEvent.ID
	if err := e.session.flushMetaLocked(); err != nil {
		e.logger.WarnContext(ctx, "failed flushing meta during digestion evaluation", "error", err)
	}
	e.session.mu.Unlock()

	go e.executeBackgroundDigestion(address, targetWindow, activeBuffer, forceDigest, targetWindowHorizonEvent.ID)
}

func (e *ChatEngine) executeBackgroundDigestion(
	address *thinkgraph.ChatAddress,
	targetWindow []thinkgraph.Event,
	activeBuffer []thinkgraph.Event,
	forceDigest bool,
	targetWindowHorizonEventID uuid.UUID,
) {
	bgCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	e.session.mu.RLock()
	activeThinkState := e.session.Meta.ActiveThinkState
	e.session.mu.RUnlock()

	thinkState, ok := e.registry.GetThinkState(activeThinkState)
	if !ok {
		e.logger.Error("Background digestion aborted: think state not found", "think_state", activeThinkState)
		return
	}

	workerModel := thinkState.Config().Roles.Worker.Model
	workerPersona := thinkState.Config().WorkerSystemPrompt()

	count, digestErr := e.digestManager.CreateBackgroundDigest(
		bgCtx,
		e.workspaceService,
		address,
		targetWindow,
		activeBuffer,
		forceDigest,
		workerModel,
		workerPersona,
	)
	if digestErr != nil {
		e.logger.Error("Background digestion failed", "error", digestErr)
		return
	}

	if count > 0 {
		e.session.mu.Lock()
		e.session.Meta.DigestionHorizon = targetWindowHorizonEventID
		if flushErr := e.session.flushMetaLocked(); flushErr != nil {
			e.logger.Warn("Failed flushing meta after digestion horizon advancement", "error", flushErr)
		}
		e.session.mu.Unlock()
		e.logger.Info("Digestion Horizon advanced", "new_horizon", targetWindowHorizonEventID, "digests_created", count)
	}
}

func (e *ChatEngine) handleTransition(ctx context.Context, address *thinkgraph.ChatAddress, targetThinkState string, ui UserInterface) error {
	targetSpace, exists := e.registry.GetThinkState(targetThinkState)
	if !exists {
		ui.OnTextChunk(fmt.Sprintf("\n\n⚠️ **System Error:** Think state '%s' does not exist in registry. Transition aborted.\n", targetThinkState))
		return fmt.Errorf("invalid transition target: %s", targetThinkState)
	}

	chatDir := filepath.Join(e.workspaceService.WorkspaceRoot(), e.session.SpaceID, "chats", address.ChatID)

	e.session.mu.Lock()
	hasInitialized := false
	for _, d := range e.session.Meta.InitializedThinkStates {
		if d == targetThinkState {
			hasInitialized = true
			break
		}
	}

	if !hasInitialized {
		if initErr := targetSpace.Initialize(ctx, chatDir); initErr != nil {
			e.session.mu.Unlock()
			return fmt.Errorf("failed to initialize domain %s: %w", targetThinkState, initErr)
		}
		e.session.Meta.InitializedThinkStates = append(e.session.Meta.InitializedThinkStates, targetThinkState)
	}

	e.session.Meta.ActiveThinkState = targetThinkState
	if err := e.session.flushMetaLocked(); err != nil {
		e.logger.WarnContext(ctx, "failed to flush meta after domain transition", "error", err)
	}
	e.session.mu.Unlock()

	if err := e.workspaceService.LogDigest(ctx, address, fmt.Sprintf("Workspace transitioned to %s domain.", targetThinkState), []string{"system"}, false, nil); err != nil {
		e.logger.WarnContext(ctx, "failed logging transition digest", "error", err)
	}
	return nil
}
