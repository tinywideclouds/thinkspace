package session

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"text/template"
	"time"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type agentResult struct {
	candidateID  string
	err          error
	record       flows.AgentRecord
	passed       bool
	trace        string
	leafEventIDs []uuid.UUID
}

type WorkerEngine struct {
	logger       *slog.Logger
	session      *ChatSession
	registry     *config.Registry
	service      *workspace.Service
	turnExecutor *TurnExecutor
	dispatcher   *thinkgraph.Dispatcher
	flow         flows.Flow
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
	return &WorkerEngine{
		logger:       logger,
		session:      session,
		registry:     registry,
		service:      service,
		turnExecutor: turnExecutor,
		dispatcher:   dispatcher,
		flow:         flow,
	}
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

			if err := engine.dispatcher.AppendEvent(ctx, chatSession, ev); err != nil {
				return nil, fmt.Errorf("failed to initialize flow anchor in graph: %w", err)
			}
			if ui != nil {
				ui.StreamGraphEvent(chatSession, thinkgraph.Node{Event: ev})
			}

			candidates, returnedJoinID, err := engine.spawnAgents(ctx, chatSession, flowAddress, space, flowContext, directive.Tasks, turnID, ui, emitter)
			if err != nil {
				return nil, err
			}
			state.Candidates = candidates
			joinNodeID = returnedJoinID

		case flows.ActionRequestStrategy:
			yieldEv := thinkgraph.NewEvent(thinkgraph.EventSystem, "Awaiting delegation strategy selection...")
			yieldEv.Edges = map[string]string{"parent_id": joinNodeID.String()}

			if err := engine.dispatcher.AppendEvent(ctx, chatSession, yieldEv); err != nil {
				engine.logger.ErrorContext(ctx, "failed to append strategy request event", "error", err)
			}

			if ui != nil {
				ui.StreamGraphEvent(chatSession, thinkgraph.Node{Event: yieldEv})
			}

			optionsNodeID = yieldEv.ID
			strategy, autoAccept := ui.ChooseNextStep()
			state.UserStrategy = strategy
			state.AutoAccept = autoAccept

			stratEv := thinkgraph.NewEvent(thinkgraph.EventSystem, fmt.Sprintf("Strategy selected: %s (auto_accept=%v)", strategy, autoAccept))
			stratEv.Edges = map[string]string{"parent_id": optionsNodeID.String()}

			if err := engine.dispatcher.AppendEvent(ctx, chatSession, stratEv); err != nil {
				engine.logger.ErrorContext(ctx, "failed to append strategy choice event", "error", err)
			}

			if ui != nil {
				ui.StreamGraphEvent(chatSession, thinkgraph.Node{Event: stratEv})
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

			if err := engine.dispatcher.AppendEvent(ctx, chatSession, yieldEv); err != nil {
				engine.logger.ErrorContext(ctx, "failed to append manual review request event", "error", err)
			}

			if ui != nil {
				ui.StreamGraphEvent(chatSession, thinkgraph.Node{Event: yieldEv})
			}

			for _, cand := range state.Candidates {
				accepted := false
				var userReason string
				if ui != nil {
					accepted, userReason = ui.ReviewCandidate(cand.CandidateID)
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
			return &signals.SignalTerminal{}, nil

		case flows.ActionFinish:
			return &signals.SignalTerminal{}, nil

		case flows.ActionError:
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
) ([]flows.BranchResult, uuid.UUID, error) {
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

	statePayload := flows.FlowExecutionState{
		Status:  flows.FlowStatusRunning,
		Summary: "Agents initializing...",
		Agents:  records,
	}
	stateBytes, err := json.Marshal(statePayload)
	if err != nil {
		return nil, uuid.Nil(), fmt.Errorf("failed to marshal initial flow execution state: %w", err)
	}

	mut := thinkgraph.StateMutation{State: stateBytes}
	mut.TargetID = flowAddress.GraphID.UUID
	mut.Type = thinkgraph.MutationState
	mut.Timestamp = time.Now().UTC()

	if err := engine.dispatcher.SaveMutation(ctx, chatSession, mut); err != nil {
		engine.logger.ErrorContext(ctx, "failed to save initial state mutation", "error", err)
	}

	if ui != nil {
		if graph := engine.service.Graph(); graph != nil {
			if nodes, err := graph.GetNodes(ctx, chatSession, []string{flowAddress.GraphID.UUID.String()}); err == nil && len(nodes) > 0 {
				ui.StreamGraphEvent(chatSession, nodes[0])
			}
		}
	}

	results := make(chan agentResult, len(tasks))

	for i, task := range tasks {
		go func(agentIndex int, t flows.SubAgentTask) {
			results <- engine.runSingleAgent(ctx, chatSession, flowAddress, space, flowContext, t, agentIndex, managerNodeID, ui, emitter)
		}(i+1, task)
	}

	for i := 0; i < len(tasks); i++ {
		res := <-results
		if res.err != nil {
			errMsgs = append(errMsgs, res.err.Error())
		} else {
			branches = append(branches, flows.BranchResult{
				CandidateID: res.candidateID,
				Passed:      res.passed,
				Trace:       res.trace,
				RawPayload:  res.record.RawPayload,
				Instruction: res.record.Instruction,
			})
			for j := range records {
				if records[j].AgentID == res.record.AgentID {
					records[j] = res.record
					break
				}
			}
			for _, id := range res.leafEventIDs {
				joinEdges = append(joinEdges, id.String())
			}
		}

		statePayload := flows.FlowExecutionState{
			Status:  flows.FlowStatusRunning,
			Summary: "Agents executing...",
			Agents:  records,
		}
		stateBytes, marshalErr := json.Marshal(statePayload)
		if marshalErr == nil {
			stateMut := thinkgraph.StateMutation{State: stateBytes}
			stateMut.TargetID = flowAddress.GraphID.UUID
			stateMut.Type = thinkgraph.MutationState
			stateMut.Timestamp = time.Now().UTC()

			if err := engine.dispatcher.SaveMutation(ctx, chatSession, stateMut); err != nil {
				engine.logger.ErrorContext(ctx, "failed to save intermediate state mutation", "error", err)
			}

			if ui != nil {
				if graph := engine.service.Graph(); graph != nil {
					if nodes, err := graph.GetNodes(ctx, chatSession, []string{flowAddress.GraphID.UUID.String()}); err == nil && len(nodes) > 0 {
						ui.StreamGraphEvent(chatSession, nodes[0])
					}
				}
			}
		} else {
			engine.logger.ErrorContext(ctx, "failed to marshal intermediate state", "error", marshalErr)
		}
	}

	if len(errMsgs) > 0 {
		return nil, uuid.Nil(), fmt.Errorf("fatal system errors aborted execution: %s", strings.Join(errMsgs, " ; "))
	}

	joinEvent := thinkgraph.NewEvent(thinkgraph.EventSystem, fmt.Sprintf("Generated %d candidate branches.", len(branches)))

	if joinEvent.Edges == nil {
		joinEvent.Edges = make(map[string]string)
	}
	for idx, edge := range joinEdges {
		joinEvent.Edges[fmt.Sprintf("parent_%d", idx)] = edge
	}

	statePayload = flows.FlowExecutionState{
		Status:  flows.FlowStatusCompleted,
		Summary: joinEvent.Content,
		Agents:  records,
	}
	stateBytes, err = json.Marshal(statePayload)
	if err != nil {
		return nil, uuid.Nil(), fmt.Errorf("failed to marshal final state payload: %w", err)
	}
	joinEvent.Data = []byte(`{}`)

	if err := engine.dispatcher.AppendEvent(ctx, flowAddress, joinEvent); err != nil {
		engine.logger.ErrorContext(ctx, "failed to append join event", "error", err)
	}

	if ui != nil {
		ui.StreamGraphEvent(flowAddress, thinkgraph.Node{Event: joinEvent, MaterializedState: stateBytes})
	}

	finalStateMut := thinkgraph.StateMutation{State: stateBytes}
	finalStateMut.TargetID = joinEvent.ID
	finalStateMut.Type = thinkgraph.MutationState
	finalStateMut.Timestamp = time.Now().UTC()

	if err := engine.dispatcher.SaveMutation(ctx, chatSession, finalStateMut); err != nil {
		engine.logger.ErrorContext(ctx, "failed to save final state mutation", "error", err)
	}

	if ui != nil {
		if graph := engine.service.Graph(); graph != nil {
			if nodes, err := graph.GetNodes(ctx, chatSession, []string{flowAddress.GraphID.UUID.String()}); err == nil && len(nodes) > 0 {
				ui.StreamGraphEvent(chatSession, nodes[0])
			}
		}
	}

	return branches, joinEvent.ID, nil
}

func (engine *WorkerEngine) runSingleAgent(
	ctx context.Context,
	chatSession *thinkgraph.ChatAddress,
	flowAddress *thinkgraph.ChatAddress,
	space spaces.ThinkState,
	flowContext flows.FlowContext,
	task flows.SubAgentTask,
	agentIndex int,
	managerNodeID uuid.UUID,
	ui UserInterface,
	emitter telemetry.Emitter,
) agentResult {
	if emitter != nil {
		emitter.Emit(task.AgentID, telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "flow_status",
			Timestamp: time.Now(),
			Message:   "Agent spawning and parsing context",
			Payload: flows.AgentTelemetry{
				FlowID:      flowContext.FlowID,
				AgentID:     task.AgentID,
				AgentIndex:  agentIndex,
				Instruction: task.Instruction,
			},
		})
	}

	candidateID := fmt.Sprintf("%s-%s-%s", chatSession.ChatID, flowContext.FlowID, task.AgentID)
	sandbox, err := engine.service.SpawnSandbox(ctx, chatSession.ChatID, candidateID)
	if err != nil {
		return agentResult{candidateID: candidateID, err: err}
	}
	defer func() {
		if tdErr := sandbox.TearDown(ctx); tdErr != nil {
			engine.logger.WarnContext(ctx, "failed to teardown sandbox", "error", tdErr)
		}
	}()

	workbench := workspace.NewWorkbench()
	workbenchItems, err := workbench.Build(ctx, sandbox, task.TargetFiles)
	if err != nil {
		return agentResult{candidateID: candidateID, err: err}
	}

	projectWorkspaceDir := engine.service.ProjectWorkspaceDir(chatSession.ProjectID)
	rawMapbook, err := space.Mapbook().GenerateLayers(ctx, projectWorkspaceDir)
	if err != nil {
		engine.logger.WarnContext(ctx, "failed to generate mapbook layers", "error", err)
	}

	stringMaps := make(map[string]string)
	for k, v := range rawMapbook {
		stringMaps[string(k)] = v
	}

	formattingConstraints := ""
	if patcher := space.Patcher(); patcher != nil {
		formattingConstraints = patcher.SystemInstructions()
	}

	bundle := cognition.ThinkBundle{
		Rules: cognition.SystemRules{
			Persona:               flowContext.BaseAgentRules,
			FormattingConstraints: formattingConstraints,
		},
		Mapbook: cognition.Mapbook{Maps: stringMaps},
		Trajectory: cognition.Trajectory{
			LongTermMemory: []string{task.ContextDigest},
			Catalyst: cognition.TrajectoryEvent{
				Role:    "user",
				Content: task.Instruction,
			},
		},
		Workbench: workbenchItems,
	}

	taskEvent := thinkgraph.NewEvent(thinkgraph.EventPrompt, task.Instruction)
	taskEvent.Metadata["agent_id"] = task.AgentID

	if taskEvent.Edges == nil {
		taskEvent.Edges = make(map[string]string)
	}
	taskEvent.Edges["parent_id"] = managerNodeID.String()

	dataBytes, err := json.Marshal(task)
	if err != nil {
		engine.logger.WarnContext(ctx, "failed to marshal sub-agent task data", "error", err)
	} else {
		taskEvent.Data = dataBytes
	}

	if err := engine.dispatcher.AppendEvent(ctx, flowAddress, taskEvent); err != nil {
		engine.logger.ErrorContext(ctx, "failed to append agent task event", "error", err)
	}

	if ui != nil {
		ui.StreamGraphEvent(flowAddress, thinkgraph.Node{Event: taskEvent})
	}

	passed, finalTrace, rawPayload, genNodeID := engine.attemptLoop(ctx, flowAddress, sandbox, bundle, flowContext, task, agentIndex, taskEvent.ID, space, ui, emitter)

	record := flows.AgentRecord{
		AgentID:           task.AgentID,
		Passed:            passed,
		Instruction:       task.Instruction,
		RawPayload:        rawPayload,
		VerificationTrace: finalTrace,
	}

	if err := sandbox.DeliverForReview(ctx); err != nil {
		return agentResult{candidateID: candidateID, err: err}
	}

	diff, err := engine.service.ReadCandidate(ctx, chatSession, candidateID)
	if err != nil {
		engine.logger.WarnContext(ctx, "failed to read candidate diff", "error", err)
	} else {
		record.StateDelta = diff
	}

	if emitter != nil {
		emitter.Emit("manager", telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "flow_complete",
			Timestamp: time.Now(),
			Message:   "Flow pipeline completed",
			Payload: flows.AgentTelemetry{
				FlowID:      flowContext.FlowID,
				AgentID:     task.AgentID,
				AgentIndex:  agentIndex,
				CandidateID: candidateID,
				Passed:      passed,
			},
		})
	}

	statusStr := "REJECTED"
	if passed {
		statusStr = "ACCEPTED"
	}
	resEvent := thinkgraph.NewEvent(workspace.EventCandidateResolution, fmt.Sprintf("Proposal %s. Reason: %s", statusStr, finalTrace))
	resEvent.Metadata["agent_id"] = task.AgentID

	if resEvent.Edges == nil {
		resEvent.Edges = make(map[string]string)
	}
	resEvent.Edges["parent_id"] = genNodeID.String()

	resPayload := workspace.CandidateResolution{
		CandidateID: candidateID,
		Accepted:    passed,
		Reason:      finalTrace,
	}
	resBytes, err := json.Marshal(resPayload)
	if err != nil {
		engine.logger.WarnContext(ctx, "failed to marshal resolution payload", "error", err)
	} else {
		resEvent.Data = resBytes
	}

	if err := engine.dispatcher.AppendEvent(ctx, flowAddress, resEvent); err != nil {
		engine.logger.ErrorContext(ctx, "failed to append agent candidate resolution event", "error", err)
	}

	if ui != nil {
		ui.StreamGraphEvent(flowAddress, thinkgraph.Node{Event: resEvent})
	}

	return agentResult{
		candidateID:  fmt.Sprintf("candidate/%s", candidateID),
		passed:       passed,
		trace:        finalTrace,
		record:       record,
		leafEventIDs: []uuid.UUID{resEvent.ID},
	}
}

func (engine *WorkerEngine) attemptLoop(
	ctx context.Context,
	flowAddress *thinkgraph.ChatAddress,
	sandbox workspace.CandidateSandbox,
	bundle cognition.ThinkBundle,
	flowContext flows.FlowContext,
	task flows.SubAgentTask,
	agentIndex int,
	taskEventID uuid.UUID,
	space spaces.ThinkState,
	ui UserInterface,
	emitter telemetry.Emitter,
) (bool, string, string, uuid.UUID) {
	var finalTrace string
	var lastPayload string
	var genNodeID uuid.UUID

	maxAttempts := 2

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 && task.RetryPrompt != "" {
			tmpl, err := template.New("retry").Parse(task.RetryPrompt)
			if err == nil {
				var buf bytes.Buffer
				if execErr := tmpl.Execute(&buf, map[string]string{"ErrorTrace": finalTrace}); execErr != nil {
					engine.logger.WarnContext(ctx, "failed to execute retry template", "error", execErr)
				} else {
					bundle.Trajectory.Catalyst.Content = buf.String()

					retry := thinkgraph.NewEvent(thinkgraph.EventPrompt, buf.String())
					retry.Metadata["agent_id"] = task.AgentID
					if retry.Edges == nil {
						retry.Edges = make(map[string]string)
					}
					retry.Edges["parent_id"] = genNodeID.String()

					if err := engine.dispatcher.AppendEvent(ctx, flowAddress, retry); err != nil {
						engine.logger.ErrorContext(ctx, "failed to append retry prompt event", "error", err)
					}

					if ui != nil {
						ui.StreamGraphEvent(flowAddress, thinkgraph.Node{Event: retry})
					}

					taskEventID = retry.ID
				}
			}
		}

		if emitter != nil {
			emitter.Emit(task.AgentID, telemetry.Event{
				ID:        uuid.NewV7(),
				Type:      "flow_status",
				Timestamp: time.Now(),
				Message:   "Executing instructions...",
				Payload: flows.AgentTelemetry{
					FlowID:     flowContext.FlowID,
					AgentID:    task.AgentID,
					AgentIndex: agentIndex,
					Attempt:    attempt,
				},
			})
		}

		mux := &streamMultiplexer{ui: ui}

		taskResponseID := uuid.NewV7()
		engine.turnExecutor.emitter = emitter

		_, err := engine.turnExecutor.Execute(ctx, flowAddress, taskResponseID, taskEventID, false, space, bundle, tools.NewRegistry(), mux)
		if err != nil {
			return false, fmt.Errorf("Execution Error: %v", err).Error(), "", uuid.Nil()
		}
		genNodeID = taskResponseID

		mut := thinkgraph.MetadataMutation{Metadata: map[string]string{"agent_id": task.AgentID}}
		mut.TargetID = taskResponseID
		mut.Type = thinkgraph.MutationMetadata
		mut.Timestamp = time.Now().UTC()

		if err := engine.service.SaveMutation(ctx, flowAddress, mut); err != nil {
			engine.logger.ErrorContext(ctx, "failed to save metadata mutation for model node", "error", err)
		}

		if graph := engine.service.Graph(); graph != nil && ui != nil {
			timeline, _ := graph.GetNodes(ctx, flowAddress, []string{taskResponseID.String()})
			if len(timeline) > 0 {
				ui.StreamGraphEvent(flowAddress, timeline[0])
			}
		}

		lastPayload = mux.buffer.String()

		if patcher := space.Patcher(); patcher != nil {
			if applyErr := patcher.Apply(ctx, sandbox, lastPayload); applyErr != nil {
				finalTrace = fmt.Sprintf("Patch Error: %v", applyErr)
				continue
			}
		}

		if applyErr := sandbox.ApplyDraft(ctx, fmt.Sprintf("attempt %d", attempt)); applyErr != nil {
			engine.logger.ErrorContext(ctx, "failed to apply sandbox draft", "error", err)
		}

		if emitter != nil {
			emitter.Emit(task.AgentID, telemetry.Event{
				ID:        uuid.NewV7(),
				Type:      "flow_status",
				Timestamp: time.Now(),
				Message:   "Running tests and verification...",
				Payload: flows.AgentTelemetry{
					FlowID:     flowContext.FlowID,
					AgentID:    task.AgentID,
					AgentIndex: agentIndex,
					Attempt:    attempt,
				},
			})
		}

		if verifier := space.Verifier(); verifier != nil {
			err = verifier.Verify(ctx, sandbox)
			if err == nil {
				return true, "", lastPayload, genNodeID
			}
			finalTrace = err.Error()

			if emitter != nil {
				emitter.Emit(task.AgentID, telemetry.Event{
					ID:        uuid.NewV7(),
					Type:      "flow_error",
					Timestamp: time.Now(),
					Message:   "Verification failed",
					Payload: flows.AgentTelemetry{
						FlowID:     flowContext.FlowID,
						AgentID:    task.AgentID,
						AgentIndex: agentIndex,
						Attempt:    attempt,
						Trace:      finalTrace,
					},
				})
			}
		} else {
			return true, "", lastPayload, genNodeID
		}
	}

	return false, finalTrace, lastPayload, genNodeID
}
