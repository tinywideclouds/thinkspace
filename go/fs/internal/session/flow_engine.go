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
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type agentResult struct {
	candidateID string
	err         error
	record      flows.AgentRecordPayload
	passed      bool
	trace       string
}

type FlowEngine struct {
	logger     *slog.Logger
	service    *workspace.Service
	executor   workspace.SubAgentExecutor
	dispatcher *thinkgraph.Dispatcher
	emitter    telemetry.Emitter
	workbench  *workspace.Workbench
}

func NewFlowEngine(
	logger *slog.Logger,
	service *workspace.Service,
	executor workspace.SubAgentExecutor,
	dispatcher *thinkgraph.Dispatcher,
	emitter telemetry.Emitter,
) *FlowEngine {
	return &FlowEngine{
		logger:     logger,
		service:    service,
		executor:   executor,
		dispatcher: dispatcher,
		emitter:    emitter,
		workbench:  workspace.NewWorkbench(),
	}
}

func (e *FlowEngine) Run(
	ctx context.Context,
	chatSession *thinkgraph.Chat,
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
			// Dispatch durable graph node for flow initialization
			ev := thinkgraph.NewEvent(flows.EventFlowStarted, flowContext.FlowSummary)
			payload := flows.FlowStartedPayload{
				FlowID:      flowContext.FlowID,
				TaskID:      chatSession.ID,
				FlowSummary: flowContext.FlowSummary,
				AgentCount:  len(directive.Tasks),
			}
			ev.Data, _ = json.Marshal(payload)
			_ = e.dispatcher.AppendEvent(ctx, chatSession, ev)

			candidates, err := e.spawnAgents(ctx, chatSession, space, flowContext, directive.Tasks)
			if err != nil {
				return err
			}
			state.Candidates = candidates

		case flows.ActionRequestStrategy:
			strategy := ui.ChooseNextStep()
			state.UserStrategy = strategy

		case flows.ActionManualReview:
			acceptedBranch := ""
			for _, cand := range state.Candidates {
				if ui.ReviewCandidate(cand.CandidateID) {
					acceptedBranch = cand.CandidateID
					break
				}
				branchName := strings.TrimPrefix(cand.CandidateID, "candidate/")
				_ = e.service.ResolveCandidate(ctx, chatSession, branchName, false, "Rejected via manual review")
			}
			state.TargetBranch = acceptedBranch

		case flows.ActionCommit:
			branchName := strings.TrimPrefix(directive.TargetBranch, "candidate/")
			return e.service.ResolveCandidate(ctx, chatSession, branchName, true, "Accepted by user")

		case flows.ActionFinish:
			return nil

		case flows.ActionError:
			return directive.Error

		default:
			return fmt.Errorf("unknown flow action: %s", directive.Action)
		}
	}

	return fmt.Errorf("flow '%s' exceeded its maximum execution limit of %d steps without resolving", flow.Name(), maxSteps)
}

func (e *FlowEngine) spawnAgents(
	ctx context.Context,
	chatSession *thinkgraph.Chat,
	space spaces.ThinkState,
	flowContext flows.FlowContext,
	tasks []flows.SubAgentTask,
) ([]flows.BranchResult, error) {

	results := make(chan agentResult, len(tasks))

	for i, task := range tasks {
		// Dispatch durable event for each agent spawned
		ev := thinkgraph.NewEvent(flows.EventAgentSpawned, task.Instruction)
		spawnPayload := flows.AgentSpawnedPayload{
			FlowID:      flowContext.FlowID,
			AgentID:     task.AgentID,
			Instruction: task.Instruction,
		}
		ev.Data, _ = json.Marshal(spawnPayload)
		_ = e.dispatcher.AppendEvent(ctx, chatSession, ev)

		go func(agentIndex int, t flows.SubAgentTask) {
			results <- e.runSingleAgent(ctx, chatSession, space, flowContext, t, agentIndex)
		}(i+1, task)
	}

	var branches []flows.BranchResult
	var records []flows.AgentRecordPayload
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
			})
			records = append(records, res.record)
		}
	}

	if len(errMsgs) > 0 {
		return nil, fmt.Errorf("fatal system errors aborted execution: %s", strings.Join(errMsgs, " ; "))
	}

	summary := fmt.Sprintf("Generated %d candidate branches.", len(branches))

	// Dispatch final durable FlowCompleted payload to the graph (replaces FlowReceipt XML)
	ev := thinkgraph.NewEvent(flows.EventFlowCompleted, summary)
	compPayload := flows.FlowCompletedPayload{
		FlowID:  flowContext.FlowID,
		TaskID:  chatSession.ID,
		Task:    "Task Execution",
		Summary: summary,
		Agents:  records,
	}
	ev.Data, _ = json.Marshal(compPayload)
	_ = e.dispatcher.AppendEvent(ctx, chatSession, ev)

	return branches, nil
}

func (e *FlowEngine) runSingleAgent(
	ctx context.Context,
	chatSession *thinkgraph.Chat,
	space spaces.ThinkState,
	flowContext flows.FlowContext,
	task flows.SubAgentTask,
	agentIndex int,
) agentResult {

	// Ephemeral Telemetry: Used purely for UI rendering, not saved
	e.emitter.Emit(task.AgentID, telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "flow_status",
		Timestamp: time.Now(),
		Message:   "Agent spawning and parsing context",
	})

	candidateID := fmt.Sprintf("%s-%s-%s", chatSession.ID, flowContext.FlowID, task.AgentID)
	sandbox, err := e.service.SpawnSandbox(ctx, chatSession.ID, candidateID)
	if err != nil {
		return agentResult{err: err}
	}
	defer sandbox.TearDown(ctx)

	workbenchItems, err := e.workbench.Build(ctx, sandbox, task.TargetFiles)
	if err != nil {
		e.emitter.Emit(task.AgentID, telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "flow_error",
			Timestamp: time.Now(),
			Message:   err.Error(),
		})
		return agentResult{err: err}
	}

	rawMapbook, _ := space.Mapbook().GenerateLayers(ctx, e.service.WorkspaceRoot())
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

	passed, finalTrace := e.attemptLoop(ctx, sandbox, bundle, flowContext, task, agentIndex, space.Verifier())

	traceBytes, _ := sandbox.ReadFile(ctx, "trace.jsonl")

	record := flows.AgentRecordPayload{
		AgentID:           task.AgentID,
		Passed:            passed,
		Instruction:       task.Instruction,
		RawPayload:        string(traceBytes),
		VerificationTrace: finalTrace,
	}

	if err := sandbox.DeliverForReview(ctx); err != nil {
		return agentResult{err: err}
	}

	diff, _ := e.service.ReadCandidate(ctx, chatSession, candidateID)
	record.StateDelta = diff

	return agentResult{
		candidateID: fmt.Sprintf("candidate/%s", candidateID),
		passed:      passed,
		trace:       finalTrace,
		record:      record,
	}
}

func (e *FlowEngine) attemptLoop(
	ctx context.Context,
	sandbox workspace.CandidateSandbox,
	bundle cognition.ThinkBundle,
	flowContext flows.FlowContext,
	task flows.SubAgentTask,
	agentIndex int,
	verifier spaces.Verifier,
) (bool, string) {

	var finalTrace string
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

		e.emitter.Emit(task.AgentID, telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "flow_status",
			Timestamp: time.Now(),
			Message:   "Executing instructions...",
		})

		err := e.executor(ctx, bundle, sandbox, agentIndex, nil)
		if err != nil {
			return false, fmt.Sprintf("Execution Error: %v", err)
		}

		_ = sandbox.ApplyDraft(ctx, fmt.Sprintf("attempt %d", attempt))

		e.emitter.Emit(task.AgentID, telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "flow_status",
			Timestamp: time.Now(),
			Message:   "Running tests and verification...",
		})

		err = verifier.Verify(ctx, sandbox)
		if err == nil {
			return true, ""
		}

		finalTrace = err.Error()

		e.emitter.Emit(task.AgentID, telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "flow_error",
			Timestamp: time.Now(),
			Message:   "Verification failed",
		})
	}

	return false, finalTrace
}
