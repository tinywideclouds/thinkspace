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
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type agentResult struct {
	candidateID string
	err         error
	record      flows.AgentRecord
	passed      bool
	trace       string
}

type streamMultiplexer struct {
	buffer    strings.Builder
	tokenChan chan<- workspace.AgentToken
	agentID   int
}

func (m *streamMultiplexer) OnTextChunk(text string) {
	m.buffer.WriteString(text)
	if m.tokenChan != nil {
		m.tokenChan <- workspace.AgentToken{AgentID: m.agentID, Text: text}
	}
}

func (m *streamMultiplexer) ConfirmTransition(req signals.SignalTransition) bool {
	return false
}

type FlowEngine struct {
	logger       *slog.Logger
	service      *workspace.Service
	turnExecutor *TurnExecutor
	dispatcher   *thinkgraph.Dispatcher
}

func NewFlowEngine(
	logger *slog.Logger,
	service *workspace.Service,
	turnExecutor *TurnExecutor,
	dispatcher *thinkgraph.Dispatcher,
) *FlowEngine {
	return &FlowEngine{
		logger:       logger,
		service:      service,
		turnExecutor: turnExecutor,
		dispatcher:   dispatcher,
	}
}

func (engine *FlowEngine) Run(
	ctx context.Context,
	chatSession *thinkgraph.ChatAddress,
	space spaces.ThinkState,
	flow flows.Flow,
	tasks []flows.SubAgentTask,
	summary string,
	ui UserInterface,
	emitter telemetry.Emitter,
) (signals.EngineSignal, error) {

	flowID := uuid.NewV7()
	flowContext := flows.FlowContext{
		FlowID:         flowID.String(),
		SpaceID:        chatSession.ProjectID,
		BaseAgentRules: space.Config().WorkerSystemPrompt(),
		FlowSummary:    summary,
	}

	state := flows.FlowState{
		InitialTasks: tasks,
	}

	maxSteps := flow.MaxSteps()

	for step := 0; step < maxSteps; step++ {
		directive := flow.Next(state)

		switch directive.Action {
		case flows.ActionSpawnAgents:
			ev := thinkgraph.Event{
				ID:        flowID,
				Type:      flows.EventFlowLifecycle,
				Timestamp: time.Now().UTC(),
				Content:   fmt.Sprintf("Flow started: %s", flowContext.FlowSummary),
			}
			configPayload := flows.FlowData{
				FlowID:     flowContext.FlowID,
				TaskID:     chatSession.ChatID,
				TaskPrompt: flowContext.FlowSummary,
			}
			ev.Data, _ = json.Marshal(configPayload)

			if err := engine.dispatcher.AppendEvent(ctx, chatSession, ev); err != nil {
				return nil, fmt.Errorf("failed to initialize flow in graph: %w", err)
			}

			candidates, err := engine.spawnAgents(ctx, chatSession, space, flowContext, directive.Tasks, flowID, ui, emitter)
			if err != nil {
				return nil, err
			}
			state.Candidates = candidates

		case flows.ActionRequestStrategy:
			strategy := ui.ChooseNextStep()
			state.UserStrategy = strategy

		case flows.ActionSynthesize:
			emitter.Emit("manager", telemetry.Event{
				ID:        uuid.NewV7(),
				Type:      "flow_status",
				Timestamp: time.Now(),
				Message:   "Preparing candidate results for Manager synthesis...",
			})

			if len(directive.Tasks) == 0 {
				return nil, fmt.Errorf("synthesis requires a refine prompt instruction")
			}

			signal := &signals.SignalSynthesize{
				OriginalInstruction: directive.Tasks[0].Instruction,
				Candidates:          state.Candidates,
			}

			if len(state.InitialTasks) > 0 {
				signal.OriginalContext = state.InitialTasks[0].ContextDigest
			}

			return signal, nil

		case flows.ActionManualReview:
			acceptedBranch := ""
			for _, cand := range state.Candidates {
				if ui.ReviewCandidate(cand.CandidateID) {
					acceptedBranch = cand.CandidateID
					break
				}
				branchName := strings.TrimPrefix(cand.CandidateID, "candidate/")
				_ = engine.service.ResolveCandidate(ctx, chatSession, branchName, false, "Rejected via manual review")
			}
			state.TargetBranch = acceptedBranch

		case flows.ActionCommit:
			branchName := strings.TrimPrefix(directive.TargetBranch, "candidate/")
			err := engine.service.ResolveCandidate(ctx, chatSession, branchName, true, "Accepted by user")
			return &signals.SignalTerminal{}, err

		case flows.ActionFinish:
			return &signals.SignalTerminal{}, nil

		case flows.ActionError:
			return nil, directive.Error

		default:
			return nil, fmt.Errorf("unknown flow action: %s", directive.Action)
		}
	}

	return nil, fmt.Errorf("flow '%s' exceeded its maximum execution limit of %d steps without resolving", flow.Name(), maxSteps)
}

func (engine *FlowEngine) spawnAgents(
	ctx context.Context,
	chatSession *thinkgraph.ChatAddress,
	space spaces.ThinkState,
	flowContext flows.FlowContext,
	tasks []flows.SubAgentTask,
	flowUUID uuid.UUID,
	ui UserInterface,
	emitter telemetry.Emitter,
) ([]flows.BranchResult, error) {

	emitter.Emit("manager", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "flow_start",
		Timestamp: time.Now(),
		Message:   "Starting Flow Execution",
		Summary:   flowContext.FlowSummary,
		Payload:   flows.FlowTelemetry{FlowID: flowContext.FlowID, TaskID: chatSession.ChatID, AgentCount: len(tasks)}.ToMap(),
	})

	var tokenChan chan<- workspace.AgentToken
	if ui != nil {
		tokenChan = ui.GetAgentTokenChannel()
	}

	results := make(chan agentResult, len(tasks))

	for i, task := range tasks {
		go func(agentIndex int, t flows.SubAgentTask) {
			results <- engine.runSingleAgent(ctx, chatSession, space, flowContext, t, agentIndex, tokenChan, emitter)
		}(i+1, task)
	}

	var branches []flows.BranchResult
	var records []flows.AgentRecord
	var errMsgs []string

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
			records = append(records, res.record)
		}

		statePayload := flows.FlowExecutionState{
			Status:  flows.FlowStatusRunning,
			Summary: "Agents executing...",
			Agents:  records,
		}
		stateBytes, _ := json.Marshal(statePayload)
		mut := thinkgraph.MutationEvent{
			TargetID:  flowUUID,
			Type:      thinkgraph.MutationUpdate,
			Timestamp: time.Now().UTC(),
			Data:      stateBytes,
		}

		_ = engine.dispatcher.SaveMutation(ctx, chatSession, mut)
	}

	if len(errMsgs) > 0 {
		return nil, fmt.Errorf("fatal system errors aborted execution: %s", strings.Join(errMsgs, " ; "))
	}

	statePayload := flows.FlowExecutionState{
		Status:  flows.FlowStatusCompleted,
		Summary: fmt.Sprintf("Generated %d candidate branches.", len(branches)),
		Agents:  records,
	}
	stateBytes, _ := json.Marshal(statePayload)
	mut := thinkgraph.MutationEvent{
		TargetID:  flowUUID,
		Type:      thinkgraph.MutationUpdate,
		Timestamp: time.Now().UTC(),
		Data:      stateBytes,
	}
	_ = engine.dispatcher.SaveMutation(ctx, chatSession, mut)

	return branches, nil
}

func (engine *FlowEngine) runSingleAgent(
	ctx context.Context,
	chatSession *thinkgraph.ChatAddress,
	space spaces.ThinkState,
	flowContext flows.FlowContext,
	task flows.SubAgentTask,
	agentIndex int,
	tokenChan chan<- workspace.AgentToken,
	emitter telemetry.Emitter,
) agentResult {

	emitter.Emit(task.AgentID, telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "flow_status",
		Timestamp: time.Now(),
		Message:   "Agent spawning and parsing context",
		Payload:   flows.AgentTelemetry{FlowID: flowContext.FlowID, AgentIndex: agentIndex, Instruction: task.Instruction}.ToMap(),
	})

	candidateID := fmt.Sprintf("%s-%s-%s", chatSession.ChatID, flowContext.FlowID, task.AgentID)
	sandbox, err := engine.service.SpawnSandbox(ctx, chatSession.ChatID, candidateID)
	if err != nil {
		return agentResult{candidateID: candidateID, err: err}
	}
	defer sandbox.TearDown(ctx)

	workbench := workspace.NewWorkbench()
	workbenchItems, err := workbench.Build(ctx, sandbox, task.TargetFiles)
	if err != nil {
		return agentResult{candidateID: candidateID, err: err}
	}

	projectWorkspaceDir := engine.service.ProjectWorkspaceDir(chatSession.ProjectID)
	rawMapbook, _ := space.Mapbook().GenerateLayers(ctx, projectWorkspaceDir)

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

	workerAddress := &thinkgraph.ChatAddress{
		ProjectID: chatSession.ProjectID,
		ChatID:    chatSession.ChatID,
		SubGraph: thinkgraph.SubGraph{
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixFlow),
			Name:    fmt.Sprintf("candidate/%s", task.AgentID),
		},
	}

	passed, finalTrace, rawPayload := engine.attemptLoop(ctx, workerAddress, sandbox, bundle, flowContext, task, agentIndex, space, tokenChan, emitter)

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

	diff, _ := engine.service.ReadCandidate(ctx, chatSession, candidateID)
	record.StateDelta = diff

	emitter.Emit("manager", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "flow_complete",
		Timestamp: time.Now(),
		Message:   "Flow pipeline completed",
		Payload:   flows.AgentTelemetry{FlowID: flowContext.FlowID, AgentIndex: agentIndex, CandidateID: candidateID, Passed: passed}.ToMap(),
	})

	return agentResult{
		candidateID: fmt.Sprintf("candidate/%s", candidateID),
		passed:      passed,
		trace:       finalTrace,
		record:      record,
	}
}

func (engine *FlowEngine) attemptLoop(
	ctx context.Context,
	workerAddress *thinkgraph.ChatAddress,
	sandbox workspace.CandidateSandbox,
	bundle cognition.ThinkBundle,
	flowContext flows.FlowContext,
	task flows.SubAgentTask,
	agentIndex int,
	space spaces.ThinkState,
	tokenChan chan<- workspace.AgentToken,
	emitter telemetry.Emitter,
) (bool, string, string) {

	var finalTrace string
	var lastPayload string
	maxAttempts := 2

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 && task.RetryPrompt != "" {
			tmpl, err := template.New("retry").Parse(task.RetryPrompt)
			if err == nil {
				var buf bytes.Buffer
				_ = tmpl.Execute(&buf, map[string]string{"ErrorTrace": finalTrace})
				bundle.Trajectory.Catalyst.Content = buf.String()
			}
		}

		emitter.Emit(task.AgentID, telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "flow_status",
			Timestamp: time.Now(),
			Message:   "Executing instructions...",
			Payload:   flows.AgentTelemetry{FlowID: flowContext.FlowID, AgentIndex: agentIndex, Attempt: attempt}.ToMap(),
		})

		mux := &streamMultiplexer{
			tokenChan: tokenChan,
			agentID:   agentIndex,
		}

		engine.turnExecutor.emitter = emitter
		_, err := engine.turnExecutor.Execute(ctx, workerAddress, space, bundle, tools.NewRegistry(), mux)
		if err != nil {
			return false, fmt.Sprintf("Execution Error: %v", err), ""
		}

		lastPayload = mux.buffer.String()

		if patcher := space.Patcher(); patcher != nil {
			if applyErr := patcher.Apply(ctx, sandbox, lastPayload); applyErr != nil {
				finalTrace = fmt.Sprintf("Patch Error: %v", applyErr)
				continue
			}
		}

		_ = sandbox.ApplyDraft(ctx, fmt.Sprintf("attempt %d", attempt))

		emitter.Emit(task.AgentID, telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "flow_status",
			Timestamp: time.Now(),
			Message:   "Running tests and verification...",
			Payload:   flows.AgentTelemetry{FlowID: flowContext.FlowID, AgentIndex: agentIndex, Attempt: attempt}.ToMap(),
		})

		if verifier := space.Verifier(); verifier != nil {
			err = verifier.Verify(ctx, sandbox)
			if err == nil {
				return true, "", lastPayload
			}
			finalTrace = err.Error()

			emitter.Emit(task.AgentID, telemetry.Event{
				ID:        uuid.NewV7(),
				Type:      "flow_error",
				Timestamp: time.Now(),
				Message:   "Verification failed",
				Payload:   flows.AgentTelemetry{FlowID: flowContext.FlowID, AgentIndex: agentIndex, Attempt: attempt, Trace: finalTrace}.ToMap(),
			})
		} else {
			return true, "", lastPayload
		}
	}

	return false, finalTrace, lastPayload
}
