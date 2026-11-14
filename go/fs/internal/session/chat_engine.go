package session

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type ChatEngine struct {
	logger           *slog.Logger
	session          *ChatSession
	registry         *config.Registry
	workspaceManager *workspace.ServiceManager
	workspaceService *workspace.Service
	bundleAssembler  *BundleAssembler
	digestManager    *DigestManager
	turnExecutor     *TurnExecutor
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
	}
}

type inlineDigestCommitter struct {
	dm           *DigestManager
	ws           *workspace.Service
	workerModel  string
	workerPrompt string
}

func (c *inlineDigestCommitter) CommitDigest(ctx context.Context, address *thinkgraph.ChatAddress, topicName string, tags []string, instruction string, contextData string) error {
	return c.dm.CreateManualDigest(ctx, c.ws, address, topicName, tags, instruction, contextData, c.workerModel, c.workerPrompt)
}

func (e *ChatEngine) Execute(ctx context.Context, address *thinkgraph.ChatAddress, turnID uuid.UUID, catalystID uuid.UUID, signal signals.EngineSignal, ui UserInterface, emitter telemetry.Emitter) (signals.EngineSignal, error) {
	switch signal.(type) {
	case *signals.SignalExecuteChat:
		return e.runStandard(ctx, address, turnID, catalystID, ui, emitter)
	default:
		return nil, fmt.Errorf("ChatEngine received unexpected signal type: %T", signal)
	}
}

func (e *ChatEngine) runStandard(ctx context.Context, address *thinkgraph.ChatAddress, turnID uuid.UUID, catalystID uuid.UUID, ui UserInterface, emitter telemetry.Emitter) (signals.EngineSignal, error) {
	e.turnExecutor.emitter = emitter

	emitter.Emit("manager", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "log",
		Timestamp: time.Now(),
		Message:   "Initializing workspace turn...",
	})

	e.session.mu.RLock()
	activeThinkState := e.session.Meta.ActiveThinkState
	digestionHorizon := e.session.Meta.DigestionHorizon
	activeBranch := e.session.Meta.ActiveBranch
	e.session.mu.RUnlock()

	thinkState, ok := e.registry.GetThinkState(activeThinkState)
	if !ok {
		return nil, fmt.Errorf("think state configuration not found: %s", activeThinkState)
	}

	e.logger.InfoContext(ctx, "assembling context bundle", "think_state", activeThinkState, "branch", activeBranch)
	bundle, graphData, err := e.bundleAssembler.Assemble(ctx, address, digestionHorizon, catalystID, activeThinkState, thinkState)
	if err != nil {
		e.logger.ErrorContext(ctx, "failed assembling bundle", "error", err)
		return nil, fmt.Errorf("assembling bundle: %w", err)
	}

	settings, err := e.workspaceManager.GetSpaceSettings(e.session.SpaceID)
	if err != nil {
		settings = workspace.SpaceSettings{ReviewPolicy: workspace.ReviewPolicySettings{Active: true, MinTokens: 200}}
	}

	e.session.mu.RLock()
	policy := e.session.reviewPolicy
	e.session.mu.RUnlock()
	if policy == nil {
		policy = NewTokenThresholdPolicy()
	}

	shouldReview := policy.ShouldReview(settings, turnID, &bundle, graphData.Focus)
	if !e.contextReview(ctx, shouldReview, turnID, &bundle, graphData, ui) {
		return &signals.SignalTerminal{}, nil
	}

	registry := tools.NewRegistry()

	var validStates []string
	for k := range e.registry.GetAllConfigs() {
		validStates = append(validStates, k)
	}
	registry.Register(tools.NewTransitionThinkStateTool(validStates))

	if thinkState.Config().Roles.Manager.Tools.QueryLens.Description != "" {
		registry.Register(tools.NewQueryLensTool(e.workspaceService))
	}

	if thinkState.Config().Roles.Manager.Tools.ProposeChange.Description != "" {
		registry.Register(tools.NewProposeChangeTool())
	}

	if thinkState.Config().Roles.Manager.Tools.CreateDigest.Description != "" {
		committer := &inlineDigestCommitter{
			dm:           e.digestManager,
			ws:           e.workspaceService,
			workerModel:  thinkState.Config().Roles.Worker.Model,
			workerPrompt: thinkState.Config().WorkerSystemPrompt(),
		}
		registry.Register(tools.NewCreateDigestTool(committer))
	}

	signal, err := e.turnExecutor.Execute(ctx, address, turnID, catalystID, true, thinkState, bundle, registry, ui)
	if err == nil {
		e.evaluateAndTriggerDigestion(ctx, address, thinkState.Config().Memory)
	}

	return signal, err
}

func (e *ChatEngine) contextReview(ctx context.Context, shouldReview bool, responseID uuid.UUID, bundle *cognition.ThinkBundle, graphData *thinkgraph.GraphData, ui UserInterface) bool {
	if !shouldReview {
		return true
	}

	turnContext := ProjectTurnContext(e.session.SpaceID, e.session.ChatID, responseID, bundle, graphData.Focus)
	decision := ui.ReviewContext(turnContext)
	if !decision.Approved {
		return false
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
		retainedCatalyst := false
		catalystIndex := len(bundle.Trajectory.WorkingMemory)

		for _, idx := range decision.RetainedHistoryIndices {
			retainedIndices[idx] = true
			if idx == catalystIndex {
				retainedCatalyst = true
			}
		}

		if bundle.Trajectory.Catalyst.Content != "" && !retainedCatalyst {
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

	return true
}

func (e *ChatEngine) evaluateAndTriggerDigestion(ctx context.Context, address *thinkgraph.ChatAddress, memoryConfig spaces.MemoryConfig) {
	graphRepository := e.workspaceManager.GraphRepository()
	if graphRepository == nil {
		return
	}

	traversalEngine := thinkgraph.NewTraversalEngine(graphRepository)
	timeline, err := traversalEngine.GetFullTimeline(ctx, address)
	if err != nil || len(timeline) == 0 {
		return
	}

	var volatileEvents []thinkgraph.Event
	horizonIndex, evalIndex := -1, -1

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

	e.session.mu.Lock()
	e.session.Meta.EvaluationHorizon = newEvalHorizonEvent.ID
	_ = e.session.flushMetaLocked()
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
		return
	}

	count, digestErr := e.digestManager.CreateBackgroundDigest(
		bgCtx,
		e.workspaceService,
		address,
		targetWindow,
		activeBuffer,
		forceDigest,
		thinkState.Config().Roles.Worker.Model,
		thinkState.Config().WorkerSystemPrompt(),
	)
	if digestErr != nil {
		return
	}

	if count > 0 {
		e.session.mu.Lock()
		e.session.Meta.DigestionHorizon = targetWindowHorizonEventID
		_ = e.session.flushMetaLocked()
		e.session.mu.Unlock()
	}
}

type streamMultiplexer struct {
	buffer strings.Builder
	ui     tools.UI
}

func (m *streamMultiplexer) OnTextChunk(chunk workspace.NodeChunk) {
	m.buffer.WriteString(chunk.Text)
	if m.ui != nil {
		m.ui.OnTextChunk(chunk)
	}
}

func (m *streamMultiplexer) ConfirmTransition(targetThinkState string, reasoning string) bool {
	return false
}
