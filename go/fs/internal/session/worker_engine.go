package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type WorkerEngine struct {
	logger      *slog.Logger
	session     *ChatSession
	registry    *config.Registry
	service     *workspace.Service
	broadcaster *GraphBroadcaster
	runner      *SubAgentRunner
	flow        flows.Flow
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
		logger:      logger,
		session:     session,
		registry:    registry,
		service:     service,
		broadcaster: broadcaster,
		runner:      runner,
		flow:        flow,
	}
}

func (engine *WorkerEngine) persistFlowState(
	ctx context.Context,
	chatSession *thinkgraph.ChatAddress,
	flowNodeID uuid.UUID,
	state flows.FlowExecutionState,
	ui UserInterface,
) {
	stateBytes, err := json.Marshal(state)
	if err != nil {
		engine.logger.ErrorContext(ctx, "failed to marshal flow execution state", "error", err)
		return
	}

	stateMut := thinkgraph.StateMutation{State: stateBytes}
	stateMut.TargetID = flowNodeID
	stateMut.Type = thinkgraph.MutationState
	stateMut.Timestamp = time.Now().UTC()

	_ = engine.broadcaster.MutateAndStream(ctx, chatSession, stateMut, flowNodeID, ui)
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

	flowID := uuid.NewV7()
	flowContext := flows.FlowContext{
		FlowID:         fmt.Sprintf("flow-%s", flowID.String()),
		SpaceID:        chatSession.ProjectID,
		BaseAgentRules: space.Config().WorkerSystemPrompt(),
		FlowSummary:    fmt.Sprintf("Executing %d sub-agent tasks", len(sig.Tasks)),
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

	var joinNodeID uuid.UUID
	var optionsNodeID uuid.UUID
	var latestRecords []flows.AgentRecord

	for step := 0; step < maxSteps; step++ {
		directive := engine.flow.Next(state)

		switch directive.Action {
		case flows.ActionSpawnAgents:
			ev := thinkgraph.NewEvent(flows.EventFlowLifecycle, fmt.Sprintf("Flow started: %s", flowContext.FlowSummary))
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

			candidates, returnedJoinID, records, err := engine.spawnAgents(ctx, chatSession, flowAddress, space, flowContext, directive.Tasks, turnID, ui, emitter)
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
				engine.logger.ErrorContext(ctx, "failed to append strategy request event", "error", err)
			}

			optionsNodeID = yieldEv.ID
			strategy, autoAccept := ui.ChooseNextStep()
			state.UserStrategy = strategy
			state.AutoAccept = autoAccept

			stratEv := thinkgraph.NewEvent(thinkgraph.EventSystem, fmt.Sprintf("Strategy selected: %s (auto_accept=%v)", strategy, autoAccept))
			stratEv.Edges = map[string]string{"parent_id": optionsNodeID.String()}
			if err := engine.broadcaster.AppendAndStream(ctx, chatSession, stratEv, ui); err != nil {
				engine.logger.ErrorContext(ctx, "failed to append strategy choice event", "error", err)
			}

		case flows.ActionSynthesize:
			if emitter != nil {
				emitter.Emit("manager", telemetry.Event{
					ID:        uuid.NewV7(),
					Type:      "flow_status",
					Timestamp: time.Now(),
					Message:   "Preparing candidate results for Manager synthesis...",
				})
			}

			if len(directive.Tasks) == 0 {
				return nil, fmt.Errorf("synthesis requires a refine prompt instruction")
			}

			signal := &signals.SignalSynthesize{
				OriginalInstruction: directive.Tasks[0].Instruction,
				Candidates:          state.Candidates,
				FlowAddress:         flowAddress,
				ParentNodeID:        optionsNodeID,
				AutoAccept:          directive.AutoAccept,
			}

			if len(state.InitialTasks) > 0 {
				signal.OriginalContext = state.InitialTasks[0].ContextDigest
			}

			return signal, nil

		case flows.ActionManualReview:
			yieldEv := thinkgraph.NewEvent(thinkgraph.EventSystem, "Awaiting manual review of candidate branches...")
			yieldEv.Edges = map[string]string{"parent_id": joinNodeID.String()}
			if err := engine.broadcaster.AppendAndStream(ctx, chatSession, yieldEv, ui); err != nil {
				engine.logger.ErrorContext(ctx, "failed to append manual review request event", "error", err)
			}

			acceptedCount := 0
			for _, cand := range state.Candidates {
				accepted := false
				var userReason string
				if ui != nil {
					accepted, userReason = ui.ReviewCandidate(cand.CandidateID)
				}
				if accepted {
					acceptedCount++
				}
				branchName := strings.TrimPrefix(cand.CandidateID, "candidate/")
				reason := strings.TrimSpace(userReason)
				if reason == "" {
					if accepted {
						reason = "Accepted by user"
					} else {
						reason = "Rejected via manual review"
					}
				}
				if err := engine.service.ResolveCandidate(ctx, chatSession, branchName, accepted, reason); err != nil {
					engine.logger.ErrorContext(ctx, "failed to resolve candidate manually", "error", err, "candidate", branchName)
				}
			}

			if acceptedCount > 0 {
				engine.persistFlowState(ctx, chatSession, flowAddress.GraphID.UUID, flows.FlowExecutionState{
					Status:  flows.FlowStatusCompleted,
					Summary: fmt.Sprintf("Accepted %d candidate branch(es) via manual review.", acceptedCount),
					Agents:  latestRecords,
				}, ui)
			} else {
				engine.persistFlowState(ctx, chatSession, flowAddress.GraphID.UUID, flows.FlowExecutionState{
					Status:  flows.FlowStatusFailed,
					Summary: "All candidate branches were rejected during manual review.",
					Agents:  latestRecords,
				}, ui)
			}

			return &signals.SignalTerminal{}, nil

		case flows.ActionFinish:
			return &signals.SignalTerminal{}, nil

		case flows.ActionError:
			if directive.Error != nil {
				engine.persistFlowState(ctx, chatSession, flowAddress.GraphID.UUID, flows.FlowExecutionState{
					Status:  flows.FlowStatusFailed,
					Summary: "Flow terminated with error.",
					Error:   directive.Error.Error(),
					Agents:  latestRecords,
				}, ui)
			}
			return nil, directive.Error

		default:
			return nil, fmt.Errorf("unknown flow action: %s", directive.Action)
		}
	}

	return nil, fmt.Errorf("flow '%s' exceeded its maximum execution limit of %d steps without resolving", engine.flow.Name(), maxSteps)
}

func (engine *WorkerEngine) spawnAgents(
	ctx context.Context,
	chatSession *thinkgraph.ChatAddress,
	flowAddress *thinkgraph.ChatAddress,
	space spaces.ThinkState,
	flowContext flows.FlowContext,
	tasks []flows.SubAgentTask,
	managerNodeID uuid.UUID,
	ui UserInterface,
	emitter telemetry.Emitter,
) ([]flows.BranchResult, uuid.UUID, []flows.AgentRecord, error) {
	engine.logger.InfoContext(ctx, "WorkerEngine spawning agents", "flow_id", flowContext.FlowID, "manager_node_id_parent", managerNodeID, "task_count", len(tasks))

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

	var branches []flows.BranchResult
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

	engine.persistFlowState(ctx, chatSession, flowAddress.GraphID.UUID, flows.FlowExecutionState{
		Status:  flows.FlowStatusRunning,
		Summary: "Agents initializing...",
		Agents:  records,
	}, ui)

	results := make(chan SubAgentResult, len(tasks))

	for i, task := range tasks {
		go func(agentIndex int, t flows.SubAgentTask) {
			results <- engine.runner.Run(ctx, SubAgentRunRequest{
				ChatSession:  chatSession,
				FlowAddress:  flowAddress,
				Space:        space,
				FlowContext:  flowContext,
				Task:         t,
				AgentIndex:   agentIndex,
				ParentNodeID: managerNodeID,
				UI:           ui,
				Emitter:      emitter,
			})
		}(i+1, task)
	}

	for i := 0; i < len(tasks); i++ {
		res := <-results
		if res.Err != nil {
			errMsgs = append(errMsgs, res.Err.Error())
		} else {
			branches = append(branches, flows.BranchResult{
				CandidateID: res.CandidateID,
				Passed:      res.Passed,
				Trace:       res.Trace,
				RawPayload:  res.Record.RawPayload,
				Instruction: res.Record.Instruction,
			})
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

		engine.persistFlowState(ctx, chatSession, flowAddress.GraphID.UUID, flows.FlowExecutionState{
			Status:  flows.FlowStatusRunning,
			Summary: "Agents executing...",
			Agents:  records,
		}, ui)
	}

	if len(errMsgs) > 0 {
		combinedErr := strings.Join(errMsgs, " ; ")
		engine.persistFlowState(ctx, chatSession, flowAddress.GraphID.UUID, flows.FlowExecutionState{
			Status:  flows.FlowStatusFailed,
			Summary: "Flow aborted due to system error.",
			Error:   combinedErr,
			Agents:  records,
		}, ui)
		return nil, uuid.Nil(), records, fmt.Errorf("fatal system errors aborted execution: %s", combinedErr)
	}

	passedCount := 0
	var failedTraces []string
	for _, b := range branches {
		if b.Passed {
			passedCount++
		} else if strings.TrimSpace(b.Trace) != "" {
			failedTraces = append(failedTraces, fmt.Sprintf("[%s]\n%s", b.CandidateID, b.Trace))
		}
	}

	var joinSummary string
	var statePayload flows.FlowExecutionState

	if passedCount == 0 {
		joinSummary = fmt.Sprintf("Generated %d candidate branch(es), but verification failed. Candidate available for review.", len(branches))
		statePayload = flows.FlowExecutionState{
			Status:  flows.FlowStatusFailed,
			Summary: joinSummary,
			Error:   strings.Join(failedTraces, "\n\n"),
			Agents:  records,
		}
	} else {
		joinSummary = fmt.Sprintf("Generated %d candidate branches (%d passed verification).", len(branches), passedCount)
		statePayload = flows.FlowExecutionState{
			Status:  flows.FlowStatusCompleted,
			Summary: joinSummary,
			Agents:  records,
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
			engine.logger.ErrorContext(ctx, "failed to append join event", "error", err)
		}
	}
	if ui != nil {
		ui.StreamGraphEvent(flowAddress, thinkgraph.Node{Event: joinEvent, MaterializedState: stateBytes})
	}

	engine.persistFlowState(ctx, chatSession, flowAddress.GraphID.UUID, statePayload, ui)

	return branches, joinEvent.ID, records, nil
}
