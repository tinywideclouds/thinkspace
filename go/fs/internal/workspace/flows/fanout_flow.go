package flows

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"text/template"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// FlowContext holds the dynamic parameters passed from the Manager/Space during execution.
type FlowContext struct {
	FlowID         string
	SpaceID        string
	BaseAgentRules string
}

// SubAgentTask represents the specific instruction assigned to a single parallel agent.
type SubAgentTask struct {
	AgentID     string
	Instruction string
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
	thread *workspace.Thread,
	space workspace.ThinkSpace,
	args map[string]any,
	flowCfg FlowConfig,
	flowCtx FlowContext,
	emitter FlowEmitter,
	executor workspace.SubAgentExecutor,
	verifier workspace.Verifier,
) (*FlowResult, error) {

	// 1. Extract Tasks from Tool Call Args
	var tasks []SubAgentTask
	if instructions, ok := args["agent_instructions"].([]any); ok {
		for i, inst := range instructions {
			tasks = append(tasks, SubAgentTask{
				AgentID:     fmt.Sprintf("agent-%d", i+1),
				Instruction: fmt.Sprintf("%v", inst),
			})
		}
	}

	// 2. Emit Flow Start
	emitter.Emit(FlowEvent{
		FlowID:     flowCtx.FlowID,
		Type:       FlowStart,
		Timestamp:  time.Now(),
		TaskID:     thread.ID,
		AgentCount: len(tasks),
	})

	f.logger.Info("starting fanout flow", "flow_id", flowCtx.FlowID, "agent_count", len(tasks))

	retryTmpl, err := template.New("retry").Parse(flowCfg.RetryPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse retry template: %w", err)
	}

	type RetryData struct {
		ErrorTrace     string
		BaseAgentRules string
	}

	// 3. Execute Sub-Agents Concurrently
	type result struct {
		candidateID string
		err         error
		record      workspace.AgentRecord
	}
	results := make(chan result, len(tasks))

	for i, task := range tasks {
		go func(agentIndex int, task SubAgentTask) {
			agentLogger := f.logger.With("flow_id", flowCtx.FlowID, "agent_id", task.AgentID)

			emitter.Emit(FlowEvent{
				FlowID:      flowCtx.FlowID,
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

			// 4. The Agent Retry Loop
			for attempt := 1; attempt <= maxAttempts; attempt++ {
				emitter.Emit(FlowEvent{
					FlowID:     flowCtx.FlowID,
					Type:       FlowStatus,
					Timestamp:  time.Now(),
					AgentID:    task.AgentID,
					AgentIndex: agentIndex,
					Status:     "executing_instructions",
					Attempt:    attempt,
				})

				prompt := task.Instruction
				if attempt > 1 {
					agentLogger.Info("compiling retry prompt with base rules")
					var buf bytes.Buffer
					_ = retryTmpl.Execute(&buf, RetryData{
						ErrorTrace:     finalTrace,
						BaseAgentRules: flowCtx.BaseAgentRules,
					})
					prompt = buf.String()
				}

				// Delegate actual environment manipulation back to the LLM layer.
				// We pass nil for the tokenChan because streaming is now handled via FlowEvents.
				err := executor(ctx, prompt, sandbox, agentIndex, nil)
				if err != nil {
					finalTrace = fmt.Sprintf("Execution Error: %v", err)
					agentLogger.Error("execution failed", "attempt", attempt, "error", err)
					continue
				}

				_ = sandbox.ApplyDraft(ctx, fmt.Sprintf("attempt %d", attempt))

				emitter.Emit(FlowEvent{
					FlowID:     flowCtx.FlowID,
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
					FlowID:     flowCtx.FlowID,
					Type:       FlowError,
					Timestamp:  time.Now(),
					AgentID:    task.AgentID,
					AgentIndex: agentIndex,
					Attempt:    attempt,
					Trace:      finalTrace,
				})
			}

			// 5. Unconditional Delivery
			if err := sandbox.DeliverForReview(ctx); err != nil {
				agentLogger.Error("failed to deliver candidate", "error", err)
				results <- result{err: err}
				return
			}

			// Capture data for the Flow Receipt
			diff, _ := service.ReadCandidate(ctx, thread, candidateID)
			traceBytes, _ := sandbox.ReadFile(ctx, "trace.jsonl") // Contains raw payload prompts & outputs

			record := workspace.AgentRecord{
				AgentID:           task.AgentID,
				Passed:            passed,
				Instruction:       task.Instruction,
				RawPayload:        string(traceBytes),
				VerificationTrace: finalTrace,
				StateDelta:        diff,
			}

			emitter.Emit(FlowEvent{
				FlowID:      flowCtx.FlowID,
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

	// 6. Aggregate Results
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

	// 7. Save Flow Receipt
	taskDescription := "FanOut Execution"
	if insts, ok := args["agent_instructions"]; ok {
		taskDescription = fmt.Sprintf("%v", insts)
	}

	receipt := workspace.FlowReceipt{
		FlowID:    flowCtx.FlowID,
		TaskID:    thread.ID,
		Timestamp: time.Now().UTC(),
		Task:      taskDescription,
		Agents:    records,
		Summary:   summary,
	}
	_ = service.SaveReceipt(ctx, thread, receipt)

	return &FlowResult{
		Branches: branches,
		Summary:  summary,
	}, nil
}
