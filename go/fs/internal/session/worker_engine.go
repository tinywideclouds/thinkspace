package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

var errSubAgentReviewAborted = errors.New("sub-agent execution aborted by user during context review")

type WorkerEngine struct {
	logger          *slog.Logger
	session         *ChatSession
	registry        *config.Registry
	service         *workspace.Service
	broadcaster     *GraphBroadcaster
	bundleAssembler *BundleAssembler
	runner          *SubAgentRunner
	flow            flows.Flow
}

func NewWorkerEngine(
	logger *slog.Logger,
	session *ChatSession,
	registry *config.Registry,
	service *workspace.Service,
	turnExecutor *TurnExecutor,
	dispatcher *thinkgraph.Dispatcher,
	flow flows.Flow,
) *WorkerEngine {
	var graphRepo thinkgraph.GraphRepository
	var workspaceRoot string
	if service != nil {
		graphRepo = service.Graph()
		workspaceRoot = service.WorkspaceRoot()
	}

	broadcaster := NewGraphBroadcaster(logger, dispatcher, graphRepo)

	var assembler *BundleAssembler
	if session != nil && session.bundleAssembler != nil {
		assembler = session.bundleAssembler
	} else {
		assembler = NewBundleAssembler(logger, graphRepo, thinkgraph.NewContextProjecter(), registry, workspaceRoot)
	}

	runner := NewSubAgentRunner(logger, service, turnExecutor, assembler, broadcaster)

	return &WorkerEngine{
		logger:          logger,
		session:         session,
		registry:        registry,
		service:         service,
		broadcaster:     broadcaster,
		bundleAssembler: assembler,
		runner:          runner,
		flow:            flow,
	}
}

func leanAgentRecords(records []flows.AgentRecord) []flows.AgentRecord {
	lean := make([]flows.AgentRecord, len(records))
	for i, r := range records {
		lean[i] = flows.AgentRecord{
			AgentID:     r.AgentID,
			Passed:      r.Passed,
			Instruction: r.Instruction,
		}
	}
	return lean
}

func (engine *WorkerEngine) Execute(ctx context.Context, chatSession *thinkgraph.ChatAddress, turnID uuid.UUID, catalystID uuid.UUID, signal signals.EngineSignal, ui UserInterface, emitter telemetry.Emitter) (signals.EngineSignal, error) {
	sig, ok := signal.(*signals.SignalDispatchFlow)
	if !ok {
		return nil, fmt.Errorf("WorkerEngine expected SignalDispatchFlow, got %T", signal)
	}

	engine.session.mu.RLock()
	activeThinkState := engine.session.Meta.ActiveThinkState
	engine.session.mu.RUnlock()

	space, exists := engine.registry.GetThinkState(activeThinkState)
	if !exists {
		return nil, fmt.Errorf("think state configuration not found: %s", activeThinkState)
	}

	flowSummary := strings.TrimSpace(sig.Intention)
	if flowSummary == "" {
		flowSummary = fmt.Sprintf("Executing %d sub-agent tasks", len(sig.Tasks))
	}

	flowID := uuid.NewV7()
	flowContext := flows.FlowContext{
		FlowID:         fmt.Sprintf("flow-%s", flowID.String()),
		SpaceID:        chatSession.ProjectID,
		BaseAgentRules: space.Config().WorkerSystemPrompt(),
		FlowSummary:    flowSummary,
	}

	state := flows.FlowState{
		InitialTasks: sig.Tasks,
	}

	maxSteps := engine.flow.MaxSteps()

	flowAddress := &thinkgraph.ChatAddress{
		ProjectID: chatSession.ProjectID,
		ChatID:    chatSession.ChatID,
		SubGraph: thinkgraph.SubGraph{
			GraphID: thinkgraph.GraphID{Prefix: thinkgraph.PrefixFlow, UUID: flowID},
			Name:    flowContext.FlowID,
		},
	}

	scope := NewFlowScope(engine.logger, engine.service, engine.broadcaster, chatSession, flowAddress, flowID)
	handedOff := false
	defer func() {
		if !handedOff {
			if finalizeErr := scope.Finalize(ctx, "Flow terminated"); finalizeErr != nil && engine.logger != nil {
				engine.logger.ErrorContext(ctx, "failed to finalize flow scope", "error", finalizeErr)
			}
		}
	}()

	var joinNodeID uuid.UUID
	var optionsNodeID uuid.UUID
	var latestRecords []flows.AgentRecord

	for step := 0; step < maxSteps; step++ {
		directive := engine.flow.Next(state)

		switch directive.Action {
		case flows.ActionSpawnAgents:
			if len(directive.Tasks) == 0 {
				return nil, fmt.Errorf("spawnAgents called with empty task list")
			}

			preparedBundles, prepErr := engine.prepareAndReviewWorkers(ctx, chatSession, flowID, space, flowContext, directive.Tasks, ui)
			if prepErr != nil {
				if errors.Is(prepErr, errSubAgentReviewAborted) {
					return &signals.SignalTerminal{}, nil
				}
				return nil, fmt.Errorf("fatal system errors aborted execution: %w", prepErr)
			}

			ev := thinkgraph.NewEvent(flows.EventFlowLifecycle, flowContext.FlowSummary)
			ev.ID = flowID
			ev.Edges = map[string]string{"parent_id": turnID.String()}

			configPayload := flows.FlowData{
				FlowID:     flowContext.FlowID,
				TaskID:     chatSession.ChatID,
				TaskPrompt: flowContext.FlowSummary,
			}
			dataBytes, err := json.Marshal(configPayload)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal flow config payload: %w", err)
			}
			ev.Data = dataBytes

			if err := engine.broadcaster.AppendAndStream(ctx, chatSession, ev, ui); err != nil {
				return nil, fmt.Errorf("failed to initialize flow anchor in graph: %w", err)
			}

			candidates, returnedJoinID, records, err := engine.spawnAgents(ctx, chatSession, flowAddress, scope, space, flowContext, directive.Tasks, preparedBundles, turnID, ui, emitter)
			if err != nil {
				return nil, err
			}
			state.Candidates = candidates
			joinNodeID = returnedJoinID
			latestRecords = records

		case flows.ActionRequestStrategy:
			yieldEv := thinkgraph.NewEvent(thinkgraph.EventSystem, "Awaiting delegation strategy selection...")
			yieldEv.Edges = map[string]string{"parent_id": joinNodeID.String()}
			if err := engine.broadcaster.AppendAndStream(ctx, chatSession, yieldEv, ui); err != nil {
				return nil, fmt.Errorf("appending strategy request event: %w", err)
			}

			optionsNodeID = yieldEv.ID
			strategy := flows.StrategySkip
			autoAccept := false
			if ui != nil {
				strategy, autoAccept = ui.ChooseNextStep()
			}
			state.UserStrategy = strategy
			state.AutoAccept = autoAccept

			stratEv := thinkgraph.NewEvent(thinkgraph.EventSystem, fmt.Sprintf("Strategy selected: %s (auto_accept=%v)", strategy, autoAccept))
			stratEv.Edges = map[string]string{"parent_id": optionsNodeID.String()}
			if err := engine.broadcaster.AppendAndStream(ctx, chatSession, stratEv, ui); err != nil {
				return nil, fmt.Errorf("appending strategy choice event: %w", err)
			}

		case flows.ActionSynthesize:
			if emitter != nil {
				emitter.Emit("manager", telemetry.Event{
					ID:        uuid.NewV7(),
					Type:      "flow_status",
					Timestamp: time.Now(),
					Message:   "Preparing candidate proposals for synthesis...",
				})
			}

			if len(directive.Tasks) == 0 {
				return nil, fmt.Errorf("synthesis requires a refine prompt instruction")
			}

			var allTargets []string
			seenTargets := make(map[string]bool)
			for _, t := range state.InitialTasks {
				for _, f := range t.TargetFiles {
					if !seenTargets[f] {
						seenTargets[f] = true
						allTargets = append(allTargets, f)
					}
				}
			}

			signal := &signals.SignalSynthesize{
				OriginalInstruction: directive.Tasks[0].Instruction,
				Candidates:          state.Candidates,
				Flow:                scope,
				FlowAddress:         flowAddress,
				ParentNodeID:        optionsNodeID,
				AutoAccept:          directive.AutoAccept,
				TargetFiles:         allTargets,
			}

			if len(state.InitialTasks) > 0 {
				signal.OriginalContext = state.InitialTasks[0].ContextDigest
			}

			handedOff = true
			return signal, nil

		case flows.ActionManualReview:
			yieldEv := thinkgraph.NewEvent(thinkgraph.EventSystem, "Awaiting manual review of candidate branches...")
			yieldEv.Edges = map[string]string{"parent_id": joinNodeID.String()}
			if err := engine.broadcaster.AppendAndStream(ctx, chatSession, yieldEv, ui); err != nil {
				return nil, fmt.Errorf("appending manual review request event: %w", err)
			}

			acceptedCount := 0
			for _, cand := range state.Candidates {
				accepted := false
				var userReason string
				if ui != nil {
					accepted, userReason = ui.ReviewCandidate(cand.BranchRef())
				}
				if accepted {
					acceptedCount++
				}
				reason := strings.TrimSpace(userReason)
				if reason == "" {
					if accepted {
						reason = "Accepted by user"
					} else {
						reason = "Rejected via manual review"
					}
				}
				if err := scope.ResolveCandidate(ctx, cand.ID, accepted, reason); err != nil {
					return nil, fmt.Errorf("resolving candidate %s manually: %w", cand.ID, err)
				}
			}

			if acceptedCount > 0 {
				if err := scope.Complete(ctx, fmt.Sprintf("Accepted %d candidate branch(es) via manual review.", acceptedCount), latestRecords, ui); err != nil {
					return nil, err
				}
			} else {
				if err := scope.Fail(ctx, "All candidate branches were rejected during manual review.", "", latestRecords, ui); err != nil {
					return nil, err
				}
			}

			return &signals.SignalTerminal{}, nil

		case flows.ActionFinish:
			if err := scope.Finalize(ctx, "Skipped by user"); err != nil {
				return nil, fmt.Errorf("finalizing skipped flow candidates: %w", err)
			}
			if err := scope.Complete(ctx, "Flow skipped by user.", latestRecords, ui); err != nil {
				return nil, err
			}
			return &signals.SignalTerminal{}, nil

		case flows.ActionError:
			if directive.Error != nil {
				if failErr := scope.Fail(ctx, "Flow terminated with error.", directive.Error.Error(), latestRecords, ui); failErr != nil && engine.logger != nil {
					engine.logger.ErrorContext(ctx, "failed recording flow error state", "error", failErr)
				}
			}
			return nil, directive.Error

		default:
			return nil, fmt.Errorf("unknown flow action: %s", directive.Action)
		}
	}

	return nil, fmt.Errorf("flow '%s' exceeded its maximum execution limit of %d steps without resolving", engine.flow.Name(), maxSteps)
}

func (engine *WorkerEngine) prepareAndReviewWorkers(
	ctx context.Context,
	chatSession *thinkgraph.ChatAddress,
	flowID uuid.UUID,
	space spaces.ThinkState,
	flowContext flows.FlowContext,
	tasks []flows.SubAgentTask,
	ui UserInterface,
) ([]cognition.ThinkBundle, error) {
	if engine.service == nil {
		return nil, fmt.Errorf("workspace service is nil")
	}

	reader := engine.service.WorkbenchReader(chatSession)
	bundles := make([]cognition.ThinkBundle, 0, len(tasks))

	for _, task := range tasks {
		items, buildErr := workspace.ReadFileContext(ctx, reader, task.TargetFiles)
		if buildErr != nil {
			return nil, buildErr
		}

		bundle := engine.bundleAssembler.AssembleWorkerBundle(
			ctx,
			chatSession.ProjectID,
			space,
			flowContext.BaseAgentRules,
			task.ContextDigest,
			task.Instruction,
			items,
		)
		bundles = append(bundles, bundle)
	}

	if ui == nil || engine.session == nil || engine.session.workspaceManager == nil {
		return bundles, nil
	}

	settings, err := engine.session.workspaceManager.GetSpaceSettings(chatSession.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("loading space settings for worker context review: %w", err)
	}

	policy := engine.session.WorkerReviewPolicy()
	combinedBundle := BuildBundledWorkerThinkBundle(tasks, bundles)
	turnContext := ProjectTurnContext(chatSession.ProjectID, chatSession.ChatID, flowID, ContextStageWorker, &combinedBundle, thinkgraph.AttentionFocus{})

	if policy.ShouldReview(settings.WorkerReviewPolicy, turnContext) {
		previewTitle := fmt.Sprintf("Sub-Agent Batch Context Preview (%d agent(s))", len(tasks))
		previewMarkdown := FormatBundlePreviewMarkdown(previewTitle, &combinedBundle)
		if previewID, saveErr := engine.service.SaveContextPreview(ctx, chatSession, previewMarkdown); saveErr != nil {
			if engine.logger != nil {
				engine.logger.WarnContext(ctx, "failed saving bundled worker context preview", "error", saveErr)
			}
		} else {
			turnContext.PreviewID = previewID
		}

		decision := ui.ReviewContext(turnContext)
		if !decision.Approved {
			return nil, errSubAgentReviewAborted
		}

		if decision.RetainedFilePaths != nil {
			retainedPaths := make(map[string]bool, len(decision.RetainedFilePaths))
			for _, p := range decision.RetainedFilePaths {
				retainedPaths[p] = true
			}
			for i := range bundles {
				var prunedWorkbench []cognition.WorkbenchItem
				for _, item := range bundles[i].Workbench {
					if retainedPaths[item.Path] {
						prunedWorkbench = append(prunedWorkbench, item)
					}
				}
				bundles[i].Workbench = prunedWorkbench
			}
		}
	}

	return bundles, nil
}

func (engine *WorkerEngine) spawnAgents(
	ctx context.Context,
	chatSession *thinkgraph.ChatAddress,
	flowAddress *thinkgraph.ChatAddress,
	scope *FlowScope,
	space spaces.ThinkState,
	flowContext flows.FlowContext,
	tasks []flows.SubAgentTask,
	preparedBundles []cognition.ThinkBundle,
	managerNodeID uuid.UUID,
	ui UserInterface,
	emitter telemetry.Emitter,
) ([]flows.Candidate, uuid.UUID, []flows.AgentRecord, error) {
	if engine.logger != nil {
		engine.logger.InfoContext(ctx, "WorkerEngine spawning agents", "flow_id", flowContext.FlowID, "manager_node_id_parent", managerNodeID, "task_count", len(tasks))
	}

	if emitter != nil {
		emitter.Emit("manager", telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "flow_start",
			Timestamp: time.Now(),
			Message:   "Starting Flow Execution",
			Summary:   flowContext.FlowSummary,
			Payload: flows.FlowTelemetry{
				FlowID:     flowContext.FlowID,
				TaskID:     chatSession.ChatID,
				AgentCount: len(tasks),
			},
		})
	}

	var candidates []flows.Candidate
	var records []flows.AgentRecord
	var errMsgs []string
	var joinEdges []string

	for _, task := range tasks {
		records = append(records, flows.AgentRecord{
			AgentID:     task.AgentID,
			Instruction: task.Instruction,
			Passed:      false,
		})
	}

	if err := scope.SetRunning(ctx, "Agents initializing...", leanAgentRecords(records), ui); err != nil {
		return nil, uuid.Nil(), records, err
	}

	results := make(chan SubAgentResult, len(tasks))

	for i, task := range tasks {
		bundleCopy := preparedBundles[i]
		go func(agentIndex int, t flows.SubAgentTask, b cognition.ThinkBundle) {
			results <- engine.runner.Run(ctx, SubAgentRunRequest{
				ChatSession:       chatSession,
				FlowAddress:       flowAddress,
				Scope:             scope,
				Space:             space,
				FlowContext:       flowContext,
				Task:              t,
				AgentIndex:        agentIndex,
				ParentNodeID:      managerNodeID,
				PrebuiltBundle:    &b,
				SkipContextReview: true,
				UI:                ui,
				Emitter:           emitter,
			})
		}(i+1, task, bundleCopy)
	}

	for i := 0; i < len(tasks); i++ {
		res := <-results
		if res.Err != nil {
			errMsgs = append(errMsgs, res.Err.Error())
		} else {
			candidates = append(candidates, res.Candidate)
			for j := range records {
				if records[j].AgentID == res.Record.AgentID {
					records[j] = res.Record
					break
				}
			}
			for _, id := range res.LeafEventIDs {
				joinEdges = append(joinEdges, id.String())
			}
		}

		if err := scope.SetRunning(ctx, "Agents executing...", leanAgentRecords(records), ui); err != nil {
			return nil, uuid.Nil(), records, err
		}
	}

	if len(errMsgs) > 0 {
		combinedErr := strings.Join(errMsgs, " ; ")
		if failErr := scope.Fail(ctx, "Flow aborted due to system error.", combinedErr, records, ui); failErr != nil && engine.logger != nil {
			engine.logger.ErrorContext(ctx, "failed recording flow abort state", "error", failErr)
		}
		return nil, uuid.Nil(), records, fmt.Errorf("fatal system errors aborted execution: %s", combinedErr)
	}

	passedCount := 0
	var failedTraces []string
	for _, c := range candidates {
		if c.Passed {
			passedCount++
		} else if strings.TrimSpace(c.Trace) != "" {
			failedTraces = append(failedTraces, fmt.Sprintf("[%s]\n%s", c.BranchRef(), c.Trace))
		}
	}

	var joinSummary string
	var statePayload flows.FlowExecutionState

	if passedCount == 0 {
		joinSummary = fmt.Sprintf("Generated %d candidate branch(es), but verification failed. Candidate available for review.", len(candidates))
		errSummary := strings.Join(failedTraces, "\n\n")
		statePayload = flows.FlowExecutionState{
			Status:  flows.FlowStatusFailed,
			Summary: joinSummary,
			Error:   errSummary,
			Agents:  records,
		}
		if err := scope.Fail(ctx, joinSummary, errSummary, records, ui); err != nil {
			return nil, uuid.Nil(), records, err
		}
	} else {
		joinSummary = fmt.Sprintf("Generated %d candidate branches (%d passed verification).", len(candidates), passedCount)
		statePayload = flows.FlowExecutionState{
			Status:  flows.FlowStatusCompleted,
			Summary: joinSummary,
			Agents:  records,
		}
		if err := scope.SetRunning(ctx, joinSummary, records, ui); err != nil {
			return nil, uuid.Nil(), records, err
		}
	}

	joinEvent := thinkgraph.NewEvent(thinkgraph.EventSystem, joinSummary)
	if joinEvent.Edges == nil {
		joinEvent.Edges = make(map[string]string)
	}
	for idx, edge := range joinEdges {
		joinEvent.Edges[fmt.Sprintf("parent_%d", idx)] = edge
	}
	joinEvent.Data = []byte(`{}`)

	stateBytes, err := json.Marshal(statePayload)
	if err != nil {
		return nil, uuid.Nil(), records, fmt.Errorf("failed to marshal final state payload: %w", err)
	}

	if engine.broadcaster.dispatcher != nil {
		if err := engine.broadcaster.dispatcher.AppendEvent(ctx, flowAddress, joinEvent); err != nil {
			return nil, uuid.Nil(), records, fmt.Errorf("appending flow join event: %w", err)
		}
	}
	if ui != nil {
		ui.StreamGraphEvent(flowAddress, thinkgraph.Node{Event: joinEvent, MaterializedState: stateBytes})
	}

	return candidates, joinEvent.ID, records, nil
}
