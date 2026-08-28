package flows

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type FanOutFlow struct {
	logger *slog.Logger
	name   string
}

func NewFanOutFlow(name string, logger *slog.Logger) *FanOutFlow {
	return &FanOutFlow{
		name:   name,
		logger: logger,
	}
}

func (f *FanOutFlow) Name() string {
	return f.name
}

type agentResult struct {
	candidateID   string
	branchName    string
	success       bool
	commitSuccess bool
	skipped       bool
}

func (f *FanOutFlow) Execute(ctx context.Context, svc *workspace.Service, thread *workspace.Thread, space workspace.ThinkSpace, args map[string]any, executor workspace.SubAgentExecutor, tokenChan chan<- workspace.AgentToken) (*FlowResult, error) {
	countFloat, ok := args["agent_count"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'agent_count' argument")
	}
	count := int(countFloat)

	rawInstructions, ok := args["agent_instructions"].([]any)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'agent_instructions' argument")
	}

	f.logger.InfoContext(ctx, "executing FanOut flow", slog.Int("agent_count", count))

	for i, inst := range rawInstructions {
		f.logger.InfoContext(ctx, "sub-agent instruction payload",
			slog.Int("agent_index", i+1),
			slog.String("instruction", fmt.Sprintf("%v", inst)),
		)
	}

	baseID := fmt.Sprintf("proposal-%d", time.Now().Unix())

	results := make([]agentResult, count)
	var wg sync.WaitGroup
	var stateMu sync.Mutex

	for i := 1; i <= count; i++ {
		wg.Add(1)
		go func(agentIdx int) {
			defer wg.Done()

			candidateID := fmt.Sprintf("%s-%d", baseID, agentIdx)
			specificInstruction := "Implement the requested feature."
			if agentIdx-1 < len(rawInstructions) {
				specificInstruction = rawInstructions[agentIdx-1].(string)
			}

			fmt.Printf("   [Agent %d] Spawning sandbox...\n", agentIdx)

			stateMu.Lock()
			sandbox, err := svc.SpawnSandbox(ctx, thread.ID, candidateID)
			stateMu.Unlock()

			if err != nil {
				f.logger.ErrorContext(ctx, "failed to spawn sandbox", "error", err, "agent", agentIdx)
				results[agentIdx-1] = agentResult{skipped: true}
				return
			}

			defer func() {
				// Safely extract the trace ledger via the virtual environment before tearing down
				if traceData, err := sandbox.ReadFile(ctx, "trace.jsonl"); err == nil {
					targetTrace := filepath.Join(svc.WorkspaceRoot(), "chats", thread.ID, fmt.Sprintf("trace-%s.jsonl", candidateID))
					_ = os.WriteFile(targetTrace, traceData, 0644)
				}

				stateMu.Lock()
				_ = sandbox.TearDown(ctx)
				stateMu.Unlock()
			}()

			maxRetries := 2
			success := false
			currentInstructions := fmt.Sprintf("%s\n\n%s", space.SubAgentSystemPrompt(), specificInstruction)

			for attempt := 1; attempt <= maxRetries; attempt++ {
				fmt.Printf("   [Agent %d] Writing code (Attempt %d/%d)...\n", agentIdx, attempt, maxRetries)

				agentCtx, agentCancel := context.WithTimeout(ctx, space.AgentTimeout())
				// Pass the pure CandidateSandbox interface to the executor
				err := executor(agentCtx, currentInstructions, sandbox, agentIdx, tokenChan)
				agentCancel()

				if err != nil {
					if agentCtx.Err() == context.DeadlineExceeded {
						fmt.Printf("   [Agent %d] ⚠️ Generation timed out.\n", agentIdx)
					}
					break
				}

				fmt.Printf("   [Agent %d] Verifying domain constraints...\n", agentIdx)
				// Pass the pure CandidateSandbox interface to the verifier
				if err := space.Verify(ctx, sandbox); err != nil {
					fmt.Printf("   [Agent %d] ⚠️ Verification failed: %v\n", agentIdx, err)
					currentInstructions = fmt.Sprintf("%s\n\nVerification failed:\n%s\nPlease fix.", specificInstruction, err.Error())
					continue
				}

				success = true
				fmt.Printf("   [Agent %d] ✅ Verification passed.\n", agentIdx)
				break
			}

			commitMsg := fmt.Sprintf("auto(candidate): proposal %s", candidateID)
			commitSuccess := true

			stateMu.Lock()
			if err := sandbox.ApplyDraft(ctx, commitMsg); err != nil {
				f.logger.ErrorContext(ctx, "failed to apply sandbox draft", "error", err, "agent", agentIdx)
				commitSuccess = false
			} else if err := sandbox.DeliverForReview(ctx); err != nil {
				f.logger.ErrorContext(ctx, "failed to deliver sandbox for review", "error", err, "agent", agentIdx)
				commitSuccess = false
			}
			stateMu.Unlock()

			results[agentIdx-1] = agentResult{
				candidateID:   candidateID,
				branchName:    "candidate/" + candidateID,
				success:       success,
				commitSuccess: commitSuccess,
				skipped:       false,
			}
		}(i)
	}

	wg.Wait()

	var branches []string
	var summaryBuilder strings.Builder
	fmt.Fprintf(&summaryBuilder, "Delegation Results (%d Agents):\n\n", count)

	for _, res := range results {
		if res.skipped {
			continue
		}
		if res.commitSuccess {
			branches = append(branches, res.branchName)
			if res.success {
				fmt.Fprintf(&summaryBuilder, "- %s (Verified: true)\n", res.branchName)
			} else {
				fmt.Fprintf(&summaryBuilder, "- %s (Verified: false - Failed Domain Constraints)\n", res.branchName)
			}
		} else {
			fmt.Fprintf(&summaryBuilder, "- %s (Git Submission Failed)\n", res.candidateID)
		}
	}

	return &FlowResult{Branches: branches, Summary: summaryBuilder.String()}, nil
}
