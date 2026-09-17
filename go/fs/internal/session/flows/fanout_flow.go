package flows

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

	"github.com/tinywideclouds.com/thinkspace/internal/assembler"
	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// FanOutPayload strictly types the agent execution context.
type FanOutPayload struct {
	FlowID      string
	TaskID      string
	AgentCount  int
	AgentIndex  int
	Instruction string
	Attempt     int
	Trace       string
	CandidateID string
	Passed      bool
}

// ToMap converts the strictly typed struct into the generic map telemetry requires.
func (p FanOutPayload) ToMap() map[string]any {
	m := make(map[string]any)
	if p.FlowID != "" {
		m["flow_id"] = p.FlowID
	}
	if p.TaskID != "" {
		m["task_id"] = p.TaskID
	}
	if p.AgentCount != 0 {
		m["agent_count"] = p.AgentCount
	}
	if p.AgentIndex != 0 {
		m["agent_index"] = p.AgentIndex
	}
	if p.Instruction != "" {
		m["instruction"] = p.Instruction
	}
	if p.Attempt != 0 {
		m["attempt"] = p.Attempt
	}
	if p.Trace != "" {
		m["trace"] = p.Trace
	}
	if p.CandidateID != "" {
		m["candidate_id"] = p.CandidateID
	}
	m["passed"] = p.Passed
	return m
}

type FlowContext struct {
	FlowID         string
	SpaceID        string
	BaseAgentRules string
	FlowSummary    string
}

type SubAgentTask struct {
	AgentID       string
	ContextDigest string
	Instruction   string
	TargetFiles   []string
}

type agentResult struct {
	candidateID string
	err         error
	record      workspace.AgentRecord
	passed      bool
}

type FanOutFlow struct {
	logger *slog.Logger
}

func NewFanOutFlow(logger *slog.Logger) *FanOutFlow {
	return &FanOutFlow{logger: logger}
}

func (f *FanOutFlow) Name() string {
	return "FanOutFlow"
}

func (f *FanOutFlow) Execute(
	ctx context.Context,
	service *workspace.Service,
	thread *chat.Thread,
	space spaces.ThinkSpace,
	args map[string]any,
	flowConfig FlowConfig,
	flowContext FlowContext,
	emitter telemetry.Emitter,
	executor workspace.SubAgentExecutor,
	verifier spaces.Verifier,
) (*FlowResult, error) {

	tasks := f.parseTasks(args)

	for _, task := range tasks {
		f.logger.Info("=== MANAGER DELEGATION REQUEST ===",
			"agent_id", task.AgentID,
			"target_files", fmt.Sprintf("%v", task.TargetFiles),
		)
	}

	emitter.Emit("manager", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "flow_start",
		Timestamp: time.Now(),
		Message:   "Starting parallel FanOut Flow",
		Summary:   flowContext.FlowSummary,
		Payload: FanOutPayload{
			FlowID:     flowContext.FlowID,
			TaskID:     thread.ID,
			AgentCount: len(tasks),
		}.ToMap(),
	})

	f.logger.Info("starting fanout flow", "flow_id", flowContext.FlowID, "agent_count", len(tasks))

	retryTemplate, err := template.New("retry").Parse(flowConfig.RetryPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse retry template: %w", err)
	}

	results := make(chan agentResult, len(tasks))

	for i, task := range tasks {
		go func(agentIndex int, task SubAgentTask) {
			results <- f.runAgentPipeline(
				ctx, service, thread, flowContext, retryTemplate, task, agentIndex, emitter, executor, verifier,
			)
		}(i+1, task)
	}

	var branches []BranchResult
	var errs []error
	var records []workspace.AgentRecord

	for i := 0; i < len(tasks); i++ {
		res := <-results
		if res.err != nil {
			errs = append(errs, res.err)
		} else {
			records = append(records, res.record)
			branches = append(branches, BranchResult{
				CandidateID: res.candidateID,
				Passed:      res.passed,
				Trace:       res.record.VerificationTrace,
			})
		}
	}

	if len(errs) > 0 {
		var errMsgs []string
		for _, e := range errs {
			errMsgs = append(errMsgs, e.Error())
		}
		return nil, fmt.Errorf("fatal system errors aborted flow: %s", strings.Join(errMsgs, " ; "))
	}

	summary := fmt.Sprintf("Generated %d candidate branches.", len(branches))

	receipt := workspace.FlowReceipt{
		FlowID:    flowContext.FlowID,
		TaskID:    thread.ID,
		Timestamp: time.Now().UTC(),
		Task:      "FanOut Execution",
		Agents:    records,
		Summary:   summary,
	}
	_ = service.SaveReceipt(ctx, thread, receipt)

	return &FlowResult{
		Branches: branches,
		Summary:  summary,
	}, nil
}

func (f *FanOutFlow) parseTasks(args map[string]any) []SubAgentTask {
	var tasks []SubAgentTask
	if rawTasks, ok := args["agent_tasks"].([]any); ok {
		for i, raw := range rawTasks {
			taskMap, isMap := raw.(map[string]any)
			if isMap {
				digest, _ := taskMap["context_digest"].(string)
				instruction, _ := taskMap["instruction"].(string)

				targetFiles := make([]string, 0)
				if rawFiles, hasFiles := taskMap["target_files"].([]any); hasFiles {
					for _, fileRaw := range rawFiles {
						if strFile, isStr := fileRaw.(string); isStr {
							targetFiles = append(targetFiles, strFile)
						}
					}
				}

				tasks = append(tasks, SubAgentTask{
					AgentID:       fmt.Sprintf("agent-%d", i+1),
					ContextDigest: digest,
					Instruction:   instruction,
					TargetFiles:   targetFiles,
				})
			} else {
				tasks = append(tasks, SubAgentTask{
					AgentID:     fmt.Sprintf("agent-%d", i+1),
					Instruction: fmt.Sprintf("%v", raw),
					TargetFiles: make([]string, 0),
				})
			}
		}
	}
	return tasks
}

func (f *FanOutFlow) runAgentPipeline(
	ctx context.Context,
	service *workspace.Service,
	thread *chat.Thread,
	flowContext FlowContext,
	retryTemplate *template.Template,
	task SubAgentTask,
	agentIndex int,
	emitter telemetry.Emitter,
	executor workspace.SubAgentExecutor,
	verifier spaces.Verifier,
) agentResult {

	agentLogger := f.logger.With("flow_id", flowContext.FlowID, "agent_id", task.AgentID)

	emitter.Emit(task.AgentID, telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "flow_spawn",
		Timestamp: time.Now(),
		Message:   "Agent spawned and parsing context",
		Payload: FanOutPayload{
			FlowID:      flowContext.FlowID,
			AgentIndex:  agentIndex,
			Instruction: task.Instruction,
		}.ToMap(),
	})

	candidateID := fmt.Sprintf("%s-%s-%s", thread.ID, flowContext.FlowID, task.AgentID)

	sandbox, err := service.SpawnSandbox(ctx, thread.ID, candidateID)
	if err != nil {
		agentLogger.Error("failed to spawn sandbox", "error", err)
		return agentResult{err: err}
	}
	defer sandbox.TearDown(ctx)

	workbench := assembler.NewWorkbench()
	workbenchText, err := workbench.Build(ctx, sandbox, task.TargetFiles)
	if err != nil {
		agentLogger.Error("manager requested invalid target files", "error", err)
		emitter.Emit(task.AgentID, telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "flow_error",
			Timestamp: time.Now(),
			Message:   err.Error(),
			Payload: FanOutPayload{
				FlowID:     flowContext.FlowID,
				AgentIndex: agentIndex,
			}.ToMap(),
		})
		return agentResult{err: err}
	}

	task.ContextDigest += workbenchText

	var passed bool
	var finalTrace string
	maxAttempts := 2

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		passed, finalTrace = f.executeSingleAttempt(
			ctx, sandbox, flowContext, retryTemplate, task, agentIndex, attempt, emitter, executor, verifier, agentLogger, finalTrace,
		)
		if passed {
			break
		}
	}

	traceBytes, _ := sandbox.ReadFile(ctx, "trace.jsonl")

	record := workspace.AgentRecord{
		AgentID:           task.AgentID,
		Passed:            passed,
		Instruction:       task.Instruction,
		RawPayload:        string(traceBytes),
		VerificationTrace: finalTrace,
		StateDelta:        "",
	}

	if err := sandbox.DeliverForReview(ctx); err != nil {
		agentLogger.Error("failed to deliver candidate", "error", err)
		return agentResult{err: err}
	}

	diff, _ := service.ReadCandidate(ctx, thread, candidateID)
	record.StateDelta = diff

	emitter.Emit("manager", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "flow_complete",
		Timestamp: time.Now(),
		Message:   "Flow pipeline completed",
		Payload: FanOutPayload{
			FlowID:      flowContext.FlowID,
			AgentIndex:  agentIndex,
			CandidateID: candidateID,
			Passed:      passed,
			Trace:       finalTrace,
		}.ToMap(),
	})

	return agentResult{candidateID: fmt.Sprintf("candidate/%s", candidateID), err: nil, passed: passed, record: record}
}

func (f *FanOutFlow) executeSingleAttempt(
	ctx context.Context,
	sandbox workspace.CandidateSandbox,
	flowContext FlowContext,
	retryTemplate *template.Template,
	task SubAgentTask,
	agentIndex int,
	attempt int,
	emitter telemetry.Emitter,
	executor workspace.SubAgentExecutor,
	verifier spaces.Verifier,
	agentLogger *slog.Logger,
	previousTrace string,
) (passed bool, finalTrace string) {

	briefing := workspace.SubAgentBriefing{
		ContextDigest: task.ContextDigest,
		Instruction:   task.Instruction,
	}

	if attempt > 1 {
		var previousAttempt string
		if traceBytes, err := sandbox.ReadFile(ctx, "trace.jsonl"); err == nil && len(traceBytes) > 0 {
			lines := strings.Split(strings.TrimSpace(string(traceBytes)), "\n")
			if len(lines) > 0 {
				lastLine := lines[len(lines)-1]
				var traceEvent struct {
					Generated string `json:"generated"`
				}
				if err := json.Unmarshal([]byte(lastLine), &traceEvent); err == nil {
					previousAttempt = traceEvent.Generated
				}
			}
		}

		type RetryData struct {
			ErrorTrace      string
			BaseAgentRules  string
			PreviousAttempt string
		}

		var buf bytes.Buffer
		_ = retryTemplate.Execute(&buf, RetryData{
			ErrorTrace:      previousTrace,
			BaseAgentRules:  flowContext.BaseAgentRules,
			PreviousAttempt: previousAttempt,
		})

		briefing.Instruction = buf.String()
	}

	emitter.Emit(task.AgentID, telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "flow_status",
		Timestamp: time.Now(),
		Message:   "executing_instructions",
		Payload: FanOutPayload{
			FlowID:     flowContext.FlowID,
			AgentIndex: agentIndex,
			Attempt:    attempt,
		}.ToMap(),
	})

	err := executor(ctx, briefing, sandbox, agentIndex, nil)
	if err != nil {
		return false, fmt.Sprintf("Execution Error: %v", err)
	}

	_ = sandbox.ApplyDraft(ctx, fmt.Sprintf("attempt %d", attempt))

	emitter.Emit(task.AgentID, telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "flow_status",
		Timestamp: time.Now(),
		Message:   "running_tests",
		Payload: FanOutPayload{
			FlowID:     flowContext.FlowID,
			AgentIndex: agentIndex,
			Attempt:    attempt,
		}.ToMap(),
	})

	err = verifier.Verify(ctx, sandbox)
	if err == nil {
		return true, ""
	}

	finalTrace = err.Error()

	emitter.Emit(task.AgentID, telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "flow_error",
		Timestamp: time.Now(),
		Message:   "verification failed",
		Payload: FanOutPayload{
			FlowID:     flowContext.FlowID,
			AgentIndex: agentIndex,
			Attempt:    attempt,
			Trace:      finalTrace,
		}.ToMap(),
	})

	return false, finalTrace
}
