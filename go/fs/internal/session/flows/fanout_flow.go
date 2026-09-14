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

	"github.com/tinywideclouds.com/thinkspace/internal/assembler"
	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// FlowContext holds the dynamic parameters passed from the Manager/Space during execution.
type FlowContext struct {
	FlowID         string
	SpaceID        string
	BaseAgentRules string
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

// -----------------------------------------------------------------------------
// 1. Orchestration & Fan-Out
// -----------------------------------------------------------------------------

func (f *FanOutFlow) Execute(
	ctx context.Context,
	service *workspace.Service,
	thread *chat.Thread,
	space spaces.ThinkSpace,
	args map[string]any,
	flowConfig FlowConfig,
	flowContext FlowContext,
	emitter FlowEmitter,
	executor workspace.SubAgentExecutor,
	verifier spaces.Verifier,
) (*FlowResult, error) {

	tasks := f.parseTasks(args)

	emitter.Emit(FlowEvent{
		FlowID:     flowContext.FlowID,
		Type:       FlowStart,
		Timestamp:  time.Now(),
		TaskID:     thread.ID,
		AgentCount: len(tasks),
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

	var branches []string
	var errs []error
	var records []workspace.AgentRecord

	for i := 0; i < len(tasks); i++ {
		res := <-results
		if res.err != nil {
			errs = append(errs, res.err)
		} else {
			branches = append(branches, res.candidateID)
			records = append(records, res.record)
		}
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("fanout completed with %d fatal system errors", len(errs))
	}

	summary := fmt.Sprintf("Successfully generated %d candidate branches.", len(branches))

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

// -----------------------------------------------------------------------------
// 2. Task Parsing
// -----------------------------------------------------------------------------

func (f *FanOutFlow) parseTasks(args map[string]any) []SubAgentTask {
	var tasks []SubAgentTask
	if rawTasks, ok := args["agent_tasks"].([]any); ok {
		for i, raw := range rawTasks {
			taskMap, isMap := raw.(map[string]any)
			if isMap {
				digest, _ := taskMap["context_digest"].(string)
				instruction, _ := taskMap["instruction"].(string)

				var targetFiles []string
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
				})
			}
		}
	}
	return tasks
}

// -----------------------------------------------------------------------------
// 3. Agent Pipeline & Sandbox Lifecycle
// -----------------------------------------------------------------------------

func (f *FanOutFlow) runAgentPipeline(
	ctx context.Context,
	service *workspace.Service,
	thread *chat.Thread,
	flowContext FlowContext,
	retryTemplate *template.Template,
	task SubAgentTask,
	agentIndex int,
	emitter FlowEmitter,
	executor workspace.SubAgentExecutor,
	verifier spaces.Verifier,
) agentResult {

	agentLogger := f.logger.With("flow_id", flowContext.FlowID, "agent_id", task.AgentID)

	emitter.Emit(FlowEvent{
		FlowID:      flowContext.FlowID,
		Type:        FlowSpawn,
		Timestamp:   time.Now(),
		AgentID:     task.AgentID,
		AgentIndex:  agentIndex,
		Instruction: task.Instruction,
	})

	// Enforce strict hierarchy: chat -> flow -> agent
	candidateID := fmt.Sprintf("%s-%s-%s", thread.ID, flowContext.FlowID, task.AgentID)

	sandbox, err := service.SpawnSandbox(ctx, thread.ID, candidateID)
	if err != nil {
		agentLogger.Error("failed to spawn sandbox", "error", err)
		return agentResult{err: err}
	}
	defer sandbox.TearDown(ctx)

	// --- THE UNIVERSAL JIT WORKBENCH ---
	// Leverage the top-level assembler to read target files and fail fast on hallucinations.
	workbench := assembler.NewWorkbench()
	workbenchText, err := workbench.Build(ctx, sandbox, task.TargetFiles)
	if err != nil {
		agentLogger.Error("manager requested invalid target files", "error", err)

		emitter.Emit(FlowEvent{
			FlowID:     flowContext.FlowID,
			Type:       FlowError,
			Timestamp:  time.Now(),
			AgentID:    task.AgentID,
			AgentIndex: agentIndex,
			Trace:      err.Error(),
		})
		return agentResult{err: err}
	}

	// Append physical target file context to the sub-agent digest
	task.ContextDigest += workbenchText
	// ----------------------------------

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

	if err := sandbox.DeliverForReview(ctx); err != nil {
		agentLogger.Error("failed to deliver candidate", "error", err)
		return agentResult{err: err}
	}

	diff, _ := service.ReadCandidate(ctx, thread, candidateID)
	traceBytes, _ := sandbox.ReadFile(ctx, "trace.jsonl")

	record := workspace.AgentRecord{
		AgentID:           task.AgentID,
		Passed:            passed,
		Instruction:       task.Instruction,
		RawPayload:        string(traceBytes),
		VerificationTrace: finalTrace,
		StateDelta:        diff,
	}

	emitter.Emit(FlowEvent{
		FlowID:      flowContext.FlowID,
		Type:        FlowComplete,
		Timestamp:   time.Now(),
		AgentID:     task.AgentID,
		AgentIndex:  agentIndex,
		CandidateID: candidateID,
		Passed:      passed,
		Trace:       finalTrace,
	})

	return agentResult{candidateID: fmt.Sprintf("candidate/%s", candidateID), err: nil, record: record}
}

// -----------------------------------------------------------------------------
// 4. Single Execution Attempt
// -----------------------------------------------------------------------------

func (f *FanOutFlow) executeSingleAttempt(
	ctx context.Context,
	sandbox workspace.CandidateSandbox,
	flowContext FlowContext,
	retryTemplate *template.Template,
	task SubAgentTask,
	agentIndex int,
	attempt int,
	emitter FlowEmitter,
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
		agentLogger.Info("compiling retry prompt with previous attempt context")

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
		agentLogger.Info("executing retry", "retry_instruction", briefing.Instruction)
	}

	emitter.Emit(FlowEvent{
		FlowID:      flowContext.FlowID,
		Type:        FlowStatus,
		Timestamp:   time.Now(),
		AgentID:     task.AgentID,
		AgentIndex:  agentIndex,
		Status:      "executing_instructions",
		Attempt:     attempt,
		Instruction: briefing.Instruction,
	})

	err := executor(ctx, briefing, sandbox, agentIndex, nil)
	if err != nil {
		agentLogger.Error("execution failed", "attempt", attempt, "error", err)
		return false, fmt.Sprintf("Execution Error: %v", err)
	}

	_ = sandbox.ApplyDraft(ctx, fmt.Sprintf("attempt %d", attempt))

	emitter.Emit(FlowEvent{
		FlowID:     flowContext.FlowID,
		Type:       FlowStatus,
		Timestamp:  time.Now(),
		AgentID:    task.AgentID,
		AgentIndex: agentIndex,
		Status:     "running_tests",
		Attempt:    attempt,
	})

	err = verifier.Verify(ctx, sandbox)
	if err == nil {
		agentLogger.Info("verification passed")
		return true, ""
	}

	finalTrace = err.Error()
	agentLogger.Warn("verification failed", "attempt", attempt, "trace", finalTrace)

	emitter.Emit(FlowEvent{
		FlowID:     flowContext.FlowID,
		Type:       FlowError,
		Timestamp:  time.Now(),
		AgentID:    task.AgentID,
		AgentIndex: agentIndex,
		Attempt:    attempt,
		Trace:      finalTrace,
	})

	return false, finalTrace
}
