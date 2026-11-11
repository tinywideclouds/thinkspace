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

// inlineDigestCommitter connects the generic tool interface to the specialized session DigestManager without creating an import cycle.
type inlineDigestCommitter struct {
	dm           *DigestManager
	ws           *workspace.Service
	workerModel  string
	workerPrompt string
}

func (c *inlineDigestCommitter) CommitDigest(ctx context.Context, address *thinkgraph.ChatAddress, topicName string, tags []string, instruction string, contextData string) error {
	return c.dm.CreateManualDigest(ctx, c.ws, address, topicName, tags, instruction, contextData, c.workerModel, c.workerPrompt)
}

func (e *ChatEngine) Run(ctx context.Context, address *thinkgraph.ChatAddress, promptText string, ui UserInterface, emitter telemetry.Emitter) (signals.EngineSignal, error) {
	e.turnExecutor.emitter = emitter

	if promptText != "" {
		e.logger.InfoContext(ctx, "processing user prompt", "chat_id", e.session.ChatID, "prompt", promptText)
		if err := e.workspaceService.AppendUserPrompt(ctx, address, promptText); err != nil {
			e.logger.ErrorContext(ctx, "failed appending user prompt to ledger", "chat_id", e.session.ChatID, "error", err)
			return nil, fmt.Errorf("appending user prompt: %w", err)
		}

		traversalEngine := thinkgraph.NewTraversalEngine(e.workspaceManager.GraphRepository())
		timeline, _, _, err := traversalEngine.GetTimelinePage(ctx, address, "", 1)
		if err == nil && len(timeline) > 0 {
			ui.StreamGraphEvent(timeline[0])
		}
	}

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
	bundle, graphData, err := e.bundleAssembler.Assemble(ctx, address, digestionHorizon, activeThinkState, thinkState)
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

	shouldReview := policy.ShouldReview(settings, &bundle, graphData.Focus)
	if !e.contextReview(ctx, shouldReview, &bundle, graphData, ui) {
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

	signal, err := e.turnExecutor.Execute(ctx, address, thinkState, bundle, registry, ui)
	if err == nil {
		e.evaluateAndTriggerDigestion(ctx, address, thinkState.Config().Memory)
	}

	return signal, err
}

func (e *ChatEngine) RunSynthesis(ctx context.Context, address *thinkgraph.ChatAddress, signal *signals.SignalSynthesize, ui UserInterface, emitter telemetry.Emitter) (signals.EngineSignal, error) {
	e.turnExecutor.emitter = emitter

	e.session.mu.RLock()
	activeThinkState := e.session.Meta.ActiveThinkState
	e.session.mu.RUnlock()

	thinkState, ok := e.registry.GetThinkState(activeThinkState)
	if !ok {
		return nil, fmt.Errorf("think state configuration not found: %s", activeThinkState)
	}

	var prompt strings.Builder
	prompt.WriteString(signal.OriginalInstruction + "\n\n")

	if signal.OriginalContext != "" {
		prompt.WriteString("### Original Task Context\n")
		prompt.WriteString(signal.OriginalContext + "\n\n")
	}

	prompt.WriteString("### Candidate Results\n")
	for _, c := range signal.Candidates {
		status := "PASSED"
		if !c.Passed {
			status = "FAILED"
		}
		prompt.WriteString(fmt.Sprintf("\n--- Candidate Branch: %s (Status: %s) ---\n", c.CandidateID, status))
		if c.Instruction != "" {
			prompt.WriteString(fmt.Sprintf("**Sub-Task Instruction:** %s\n", c.Instruction))
		}
		if c.Trace != "" {
			prompt.WriteString(fmt.Sprintf("**Verification Error Trace:**\n```\n%s\n```\n", c.Trace))
		}
		if c.RawPayload != "" {
			prompt.WriteString(fmt.Sprintf("**Raw Sub-Agent Output:**\n```\n%s\n```\n", c.RawPayload))
		}
	}

	synthAddress := &thinkgraph.ChatAddress{
		ProjectID: address.ProjectID,
		ChatID:    address.ChatID,
		SubGraph: thinkgraph.SubGraph{
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixFlow),
			Name:    "synthesis",
		},
	}

	projectWorkspaceDir := e.workspaceService.ProjectWorkspaceDir(address.ProjectID)
	rawMapbook, _ := thinkState.Mapbook().GenerateLayers(ctx, projectWorkspaceDir)
	stringMaps := make(map[string]string)
	for k, v := range rawMapbook {
		stringMaps[string(k)] = v
	}

	bundle := cognition.ThinkBundle{
		Rules:   cognition.SystemRules{Persona: thinkState.Config().ManagerSystemPrompt()},
		Mapbook: cognition.Mapbook{Maps: stringMaps},
		Trajectory: cognition.Trajectory{
			Catalyst: cognition.TrajectoryEvent{
				Role:    "user",
				Content: prompt.String(),
			},
		},
		Workbench: []cognition.WorkbenchItem{},
	}

	// 1. Spawn a dedicated Sandbox to apply the synthesized result
	synthCandidateID := fmt.Sprintf("%s-synthesis-%s", address.ChatID, uuid.NewV7().String()[:8])
	sandbox, err := e.workspaceService.SpawnSandbox(ctx, address.ChatID, synthCandidateID)
	if err != nil {
		return nil, fmt.Errorf("failed to spawn synthesis sandbox: %w", err)
	}
	defer sandbox.TearDown(ctx)

	// 2. Multiplex the stream so we can intercept the text output
	mux := &streamMultiplexer{ui: ui}

	_, err = e.turnExecutor.Execute(ctx, synthAddress, thinkState, bundle, tools.NewRegistry(), mux)
	if err != nil {
		return nil, fmt.Errorf("synthesis execution failed: %w", err)
	}

	lastPayload := mux.buffer.String()

	// 3. Apply Patch and Verify
	if patcher := thinkState.Patcher(); patcher != nil {
		if applyErr := patcher.Apply(ctx, sandbox, lastPayload); applyErr != nil {
			e.logger.WarnContext(ctx, "failed to apply synthesis patch", "error", applyErr)
		}
	}

	_ = sandbox.ApplyDraft(ctx, "synthesis")

	passed := true
	var finalTrace string
	if verifier := thinkState.Verifier(); verifier != nil {
		if vErr := verifier.Verify(ctx, sandbox); vErr != nil {
			passed = false
			finalTrace = vErr.Error()
		}
	}

	// 4. Deliver and Request Manual Review
	if err := sandbox.DeliverForReview(ctx); err != nil {
		return nil, fmt.Errorf("failed to deliver synthesis sandbox for review: %w", err)
	}

	diff, _ := e.workspaceService.ReadCandidate(ctx, address, synthCandidateID)

	emitter.Emit("manager", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "flow_complete",
		Timestamp: time.Now(),
		Message:   "Synthesis pipeline completed",
		Payload:   map[string]any{"candidate_id": synthCandidateID, "passed": passed, "trace": finalTrace, "diff": diff},
	})

	uiBranchName := fmt.Sprintf("candidate/%s", synthCandidateID)
	accepted := ui.ReviewCandidate(uiBranchName)

	reason := "Accepted by user"
	if !accepted {
		reason = "Rejected by user"
	}

	_ = e.workspaceService.ResolveCandidate(ctx, address, synthCandidateID, accepted, reason)

	e.evaluateAndTriggerDigestion(ctx, address, thinkState.Config().Memory)

	return &signals.SignalTerminal{}, err
}

func (e *ChatEngine) contextReview(ctx context.Context, shouldReview bool, bundle *cognition.ThinkBundle, graphData *thinkgraph.GraphData, ui UserInterface) bool {
	if !shouldReview {
		return true
	}

	turnContext := ProjectTurnContext(e.session.SpaceID, e.session.ChatID, bundle, graphData.Focus)
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
