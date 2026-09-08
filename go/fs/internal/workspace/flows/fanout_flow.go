package flows

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"text/template"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/chat"
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
	space workspace.ThinkSpace,
	args map[string]any,
	flowConfig FlowConfig,
	flowContext FlowContext,
	emitter FlowEmitter,
	executor workspace.SubAgentExecutor,
	verifier workspace.Verifier,
) (*FlowResult, error) {

	var tasks []SubAgentTask
	if rawTasks, ok := args["agent_tasks"].([]any); ok {
		for i, raw := range rawTasks {
			taskMap, isMap := raw.(map[string]any)
			if isMap {
				digest, _ := taskMap["context_digest"].(string)
				instruction, _ := taskMap["instruction"].(string)
				tasks = append(tasks, SubAgentTask{
					AgentID:       fmt.Sprintf("agent-%d", i+1),
					ContextDigest: digest,
					Instruction:   instruction,
				})
			} else {
				// Fallback if the model outputs a flat string array instead of objects
				tasks = append(tasks, SubAgentTask{
					AgentID:     fmt.Sprintf("agent-%d", i+1),
					Instruction: fmt.Sprintf("%v", raw),
				})
			}
		}
	}

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

	type RetryData struct {
		ErrorTrace     string
		BaseAgentRules string
	}

	type result struct {
		candidateID string
		err         error
		record      workspace.AgentRecord
	}
	results := make(chan result, len(tasks))

	for i, task := range tasks {
		go func(agentIndex int, task SubAgentTask) {
			agentLogger := f.logger.With("flow_id", flowContext.FlowID, "agent_id", task.AgentID)

			emitter.Emit(FlowEvent{
				FlowID:      flowContext.FlowID,
				Type:        FlowSpawn,
				Timestamp:   time.Now(),
				AgentID:     task.AgentID,
				AgentIndex:  agentIndex,
				Instruction: task.Instruction,
			})

			candidateID := fmt.Sprintf("%s-%s", thread.ID, task.AgentID)
			sandbox, err := service.SpawnSandbox(ctx, thread.ID, candidateID)
			if err != nil {
				agentLogger.Error("failed to spawn sandbox", "error", err)
				results <- result{err: err}
				return
			}
			defer sandbox.TearDown(ctx)

			var passed bool
			var finalTrace string
			maxAttempts := 2

			for attempt := 1; attempt <= maxAttempts; attempt++ {
				emitter.Emit(FlowEvent{
					FlowID:     flowContext.FlowID,
					Type:       FlowStatus,
					Timestamp:  time.Now(),
					AgentID:    task.AgentID,
					AgentIndex: agentIndex,
					Status:     "executing_instructions",
					Attempt:    attempt,
				})

				briefing := workspace.SubAgentBriefing{
					ContextDigest: task.ContextDigest,
					Instruction:   task.Instruction,
				}

				if attempt > 1 {
					agentLogger.Info("compiling retry prompt with base rules")
					var buf bytes.Buffer
					_ = retryTemplate.Execute(&buf, RetryData{
						ErrorTrace:     finalTrace,
						BaseAgentRules: flowContext.BaseAgentRules,
					})
					// Override the instruction with the retry payload
					briefing.Instruction = buf.String()
				}

				err := executor(ctx, briefing, sandbox, agentIndex, nil)
				if err != nil {
					finalTrace = fmt.Sprintf("Execution Error: %v", err)
					agentLogger.Error("execution failed", "attempt", attempt, "error", err)
					continue
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
					passed = true
					agentLogger.Info("verification passed")
					break
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
			}

			if err := sandbox.DeliverForReview(ctx); err != nil {
				agentLogger.Error("failed to deliver candidate", "error", err)
				results <- result{err: err}
				return
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

			results <- result{candidateID: fmt.Sprintf("candidate/%s", candidateID), err: nil, record: record}
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
