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

// workerUI buffers the streaming LLM output so the FlowEngine can extract the final payload
// and pass it to the domain Patcher for physical file mutations.
type workerUI struct {
	UserInterface
	buffer    strings.Builder
	tokenChan chan<- workspace.AgentToken
	agentID   int
}

func (w *workerUI) OnTextChunk(text string) {
	w.buffer.WriteString(text)
	if w.tokenChan != nil {
		w.tokenChan <- workspace.AgentToken{AgentID: w.agentID, Text: text}
	}
}

type FlowEngine struct {
	logger       *slog.Logger
	service      *workspace.Service
	turnExecutor *TurnExecutor
	dispatcher   *thinkgraph.Dispatcher
	emitter      telemetry.Emitter
	workbench    *workspace.Workbench
}

func NewFlowEngine(
	logger *slog.Logger,
	service *workspace.Service,
	turnExecutor *TurnExecutor,
	dispatcher *thinkgraph.Dispatcher,
	emitter telemetry.Emitter,
) *FlowEngine {
	return &FlowEngine{
		logger:       logger,
		service:      service,
		turnExecutor: turnExecutor,
		dispatcher:   dispatcher,
		emitter:      emitter,
		workbench:    workspace.NewWorkbench(),
	}
}

func (engine *FlowEngine) Run(
	ctx context.Context,
	chatSession *thinkgraph.ChatAddress,
	space spaces.ThinkState,
	flow flows.Flow,
	args map[string]any,
	flowContext flows.FlowContext,
	ui UserInterface,
) error {
	state := flows.FlowState{
		LLMArguments: args,
	}

	maxSteps := flow.MaxSteps()

	for step := 0; step < maxSteps; step++ {
		directive := flow.Next(state)

		switch directive.Action {
		case flows.ActionSpawnAgents:
			id, err := uuid.Parse(flowContext.FlowID)
			if err != nil {
				return fmt.Errorf("invalid flow ID format: %w", err)
			}

			ev := thinkgraph.Event{
				ID:        id,
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
				engine.logger.ErrorContext(ctx, "failed to append flow lifecycle event", "error", err)
				return fmt.Errorf("failed to initialize flow in graph: %w", err)
			}

			candidates, err := engine.spawnAgents(ctx, chatSession, space, flowContext, directive.Tasks, id, ui)
			if err != nil {
				return err
			}
			state.Candidates = candidates

		case flows.ActionRequestStrategy:
			strategy := ui.ChooseNextStep()
			state.UserStrategy = strategy

		case flows.ActionSynthesize:
			engine.logger.InfoContext(ctx, "synthesizing candidates into refined solution")
			engine.emitter.Emit("manager", telemetry.Event{
				ID:        uuid.NewV7(),
				Type:      "flow_status",
				Timestamp: time.Now(),
				Message:   "Preparing candidate dossier for Manager synthesis...",
			})

			if len(directive.Tasks) == 0 {
				return fmt.Errorf("synthesis requires a refine prompt instruction")
			}

			var dossier strings.Builder
			dossier.WriteString(directive.Tasks[0].Instruction + "\n\n")

			if rawTasks, ok := state.LLMArguments["agent_tasks"].([]any); ok && len(rawTasks) > 0 {
				if tm, isMap := rawTasks[0].(map[string]any); isMap {
					if originalDigest, ok := tm["context_digest"].(string); ok && originalDigest != "" {
						dossier.WriteString("### Original Task Context\n")
						dossier.WriteString(originalDigest + "\n\n")
					}
				}
			}

			dossier.WriteString("### Candidate Dossier\n")
			for _, c := range state.Candidates {
				status := "PASSED"
				if !c.Passed {
					status = "FAILED"
				}
				dossier.WriteString(fmt.Sprintf("\n--- Candidate Branch: %s (Status: %s) ---\n", c.CandidateID, status))
				if c.Instruction != "" {
					dossier.WriteString(fmt.Sprintf("**Sub-Task Instruction:** %s\n", c.Instruction))
				}
				if c.Trace != "" {
					dossier.WriteString(fmt.Sprintf("**Verification Error Trace:**\n```\n%s\n```\n", c.Trace))
				}
				if c.RawPayload != "" {
					dossier.WriteString(fmt.Sprintf("**Raw Sub-Agent Output:**\n```\n%s\n```\n", c.RawPayload))
				}
			}

			synthAddress := &thinkgraph.ChatAddress{
				ProjectID: chatSession.ProjectID,
				ChatID:    chatSession.ChatID,
				SubGraph: thinkgraph.SubGraph{
					GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixFlow),
					Name:    "synthesis",
				},
			}

			bundle := cognition.ThinkBundle{
				Rules: cognition.SystemRules{Persona: space.Config().ManagerSystemPrompt()},
				Trajectory: cognition.Trajectory{
					Catalyst: cognition.TrajectoryEvent{
						Role:    "user",
						Content: dossier.String(),
					},
				},
			}

			return engine.turnExecutor.Execute(ctx, synthAddress, space, bundle, tools.NewRegistry(), ui)

		case flows.ActionManualReview:
			acceptedBranch := ""
			for _, cand := range state.Candidates {
				if ui.ReviewCandidate(cand.CandidateID) {
					acceptedBranch = cand.CandidateID
					break
				}
				branchName := strings.TrimPrefix(cand.CandidateID, "candidate/")
				if err := engine.service.ResolveCandidate(ctx, chatSession, branchName, false, "Rejected via manual review"); err != nil {
					engine.logger.ErrorContext(ctx, "failed to reject candidate", "candidate", branchName, "error", err)
				}
			}
			state.TargetBranch = acceptedBranch

		case flows.ActionCommit:
			branchName := strings.TrimPrefix(directive.TargetBranch, "candidate/")
			return engine.service.ResolveCandidate(ctx, chatSession, branchName, true, "Accepted by user")

		case flows.ActionFinish:
			return nil

		case flows.ActionError:
			engine.logger.ErrorContext(ctx, "flow directive returned error",
				"flow", flow.Name(),
				"error", directive.Error,
			)
			for _, c := range state.Candidates {
				engine.logger.ErrorContext(ctx, "candidate verification failure detail",
					"candidate_id", c.CandidateID,
					"passed", c.Passed,
					"trace", c.Trace,
				)
			}
			return directive.Error

		default:
			return fmt.Errorf("unknown flow action: %s", directive.Action)
		}
	}

	return fmt.Errorf("flow '%s' exceeded its maximum execution limit of %d steps without resolving", flow.Name(), maxSteps)
}

func (engine *FlowEngine) spawnAgents(
	ctx context.Context,
	chatSession *thinkgraph.ChatAddress,
	space spaces.ThinkState,
	flowContext flows.FlowContext,
	tasks []flows.SubAgentTask,
	flowUUID uuid.UUID,
	ui UserInterface,
) ([]flows.BranchResult, error) {

	engine.emitter.Emit("manager", telemetry.Event{
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
			results <- engine.runSingleAgent(ctx, chatSession, space, flowContext, t, agentIndex, tokenChan, ui)
		}(i+1, task)
	}

	var branches []flows.BranchResult
	var records []flows.AgentRecord
	var errMsgs []string

	for i := 0; i < len(tasks); i++ {
		res := <-results
		if res.err != nil {
			engine.logger.ErrorContext(ctx, "agent execution encountered internal error",
				"candidate_id", res.candidateID,
				"error", res.err,
			)
			errMsgs = append(errMsgs, res.err.Error())
		} else {
			engine.logger.InfoContext(ctx, "agent execution completed",
				"candidate_id", res.candidateID,
				"passed", res.passed,
				"has_trace", res.trace != "",
			)
			if !res.passed && res.trace != "" {
				engine.logger.WarnContext(ctx, "agent verification failed trace",
					"candidate_id", res.candidateID,
					"trace", res.trace,
				)
			}
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

		if err := engine.dispatcher.SaveMutation(ctx, chatSession, mut); err != nil {
			engine.logger.ErrorContext(ctx, "failed to save flow mutation during execution", "flow_id", flowUUID, "error", err)
		}
	}

	if len(errMsgs) > 0 {
		statePayload := flows.FlowExecutionState{
			Status:  flows.FlowStatusFailed,
			Summary: "System errors aborted execution.",
			Error:   strings.Join(errMsgs, " ; "),
			Agents:  records,
		}
		stateBytes, _ := json.Marshal(statePayload)
		mut := thinkgraph.MutationEvent{
			TargetID:  flowUUID,
			Type:      thinkgraph.MutationUpdate,
			Timestamp: time.Now().UTC(),
			Data:      stateBytes,
		}

		if err := engine.dispatcher.SaveMutation(ctx, chatSession, mut); err != nil {
			engine.logger.ErrorContext(ctx, "failed to save failure mutation", "flow_id", flowUUID, "error", err)
		}

		return nil, fmt.Errorf("fatal system errors aborted execution: %s", strings.Join(errMsgs, " ; "))
	}

	summary := fmt.Sprintf("Generated %d candidate branches.", len(branches))

	statePayload := flows.FlowExecutionState{
		Status:  flows.FlowStatusCompleted,
		Summary: summary,
		Agents:  records,
	}
	stateBytes, _ := json.Marshal(statePayload)
	mut := thinkgraph.MutationEvent{
		TargetID:  flowUUID,
		Type:      thinkgraph.MutationUpdate,
		Timestamp: time.Now().UTC(),
		Data:      stateBytes,
	}

	if err := engine.dispatcher.SaveMutation(ctx, chatSession, mut); err != nil {
		engine.logger.ErrorContext(ctx, "failed to save final completion mutation", "flow_id", flowUUID, "error", err)
	}

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
	ui UserInterface,
) agentResult {

	engine.emitter.Emit(task.AgentID, telemetry.Event{
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

	workbenchItems, err := engine.workbench.Build(ctx, sandbox, task.TargetFiles)
	if err != nil {
		engine.emitter.Emit(task.AgentID, telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "flow_error",
			Timestamp: time.Now(),
			Message:   err.Error(),
			Payload:   flows.AgentTelemetry{FlowID: flowContext.FlowID, AgentIndex: agentIndex}.ToMap(),
		})
		return agentResult{candidateID: candidateID, err: err}
	}

	rawMapbook, _ := space.Mapbook().GenerateLayers(ctx, engine.service.WorkspaceRoot())
	stringMaps := make(map[string]string)
	for k, v := range rawMapbook {
		stringMaps[string(k)] = v
	}

	bundle := cognition.ThinkBundle{
		Rules: cognition.SystemRules{
			Persona: flowContext.BaseAgentRules,
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

	passed, finalTrace, rawPayload := engine.attemptLoop(ctx, workerAddress, sandbox, bundle, flowContext, task, agentIndex, space, tokenChan, ui)

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

	engine.emitter.Emit("manager", telemetry.Event{
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
	ui UserInterface,
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

		engine.emitter.Emit(task.AgentID, telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "flow_status",
			Timestamp: time.Now(),
			Message:   "Executing instructions...",
			Payload:   flows.AgentTelemetry{FlowID: flowContext.FlowID, AgentIndex: agentIndex, Attempt: attempt}.ToMap(),
		})

		engine.logger.DebugContext(ctx, "sub-agent execution starting",
			"agent_id", task.AgentID,
			"attempt", attempt,
		)

		wUI := &workerUI{
			UserInterface: ui,
			tokenChan:     tokenChan,
			agentID:       agentIndex,
		}

		emptyRegistry := tools.NewRegistry()

		err := engine.turnExecutor.Execute(ctx, workerAddress, space, bundle, emptyRegistry, wUI)
		if err != nil {
			engine.logger.ErrorContext(ctx, "sub-agent LLM executor failed",
				"agent_id", task.AgentID,
				"attempt", attempt,
				"error", err,
			)
			return false, fmt.Sprintf("Execution Error: %v", err), ""
		}

		lastPayload = wUI.buffer.String()

		if patcher := space.Patcher(); patcher != nil {
			if applyErr := patcher.Apply(ctx, sandbox, lastPayload); applyErr != nil {
				engine.logger.ErrorContext(ctx, "failed to apply patch", "agent", task.AgentID, "error", applyErr)
				finalTrace = fmt.Sprintf("Patch Error: %v", applyErr)
				continue
			}
		}

		if err := sandbox.ApplyDraft(ctx, fmt.Sprintf("attempt %d", attempt)); err != nil {
			engine.logger.ErrorContext(ctx, "failed to apply draft", "agent", task.AgentID, "error", err)
		}

		engine.emitter.Emit(task.AgentID, telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "flow_status",
			Timestamp: time.Now(),
			Message:   "Running tests and verification...",
			Payload:   flows.AgentTelemetry{FlowID: flowContext.FlowID, AgentIndex: agentIndex, Attempt: attempt}.ToMap(),
		})

		if verifier := space.Verifier(); verifier != nil {
			err = verifier.Verify(ctx, sandbox)
			if err == nil {
				engine.logger.InfoContext(ctx, "sub-agent passed verification",
					"agent_id", task.AgentID,
					"attempt", attempt,
				)
				return true, "", lastPayload
			}
			finalTrace = err.Error()

			engine.logger.WarnContext(ctx, "sub-agent attempt failed verification",
				"agent_id", task.AgentID,
				"attempt", attempt,
				"error_trace", finalTrace,
			)

			engine.emitter.Emit(task.AgentID, telemetry.Event{
				ID:        uuid.NewV7(),
				Type:      "flow_error",
				Timestamp: time.Now(),
				Message:   "Verification failed",
				Payload:   flows.AgentTelemetry{FlowID: flowContext.FlowID, AgentIndex: agentIndex, Attempt: attempt, Trace: finalTrace}.ToMap(),
			})
		} else {
			// Without a strict verifier, we blindly pass the candidate
			return true, "", lastPayload
		}
	}

	return false, finalTrace, lastPayload
}
