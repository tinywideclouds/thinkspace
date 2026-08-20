package workspace

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
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

// agentResult holds the outcome of a single concurrent agent execution
type agentResult struct {
	candidateID   string
	branchName    string
	success       bool
	commitSuccess bool
	skipped       bool
}

func (f *FanOutFlow) Execute(ctx context.Context, svc *Service, thread *Thread, space ThinkSpace, args map[string]any, executor SubAgentExecutor) (*FlowResult, error) {
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
	var gitMu sync.Mutex // Protects Git operations on the main repository

	// Launch all agents concurrently
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

			// Mutex: Prevent Git lock collisions when creating worktrees/clones
			fmt.Printf("   [Agent %d] 🔒 Requesting Git lock to spawn sandbox...\n", agentIdx)
			gitMu.Lock()
			sandboxDir, err := svc.state.SpawnSandbox(ctx, svc.workspaceRoot, thread.ID, candidateID)
			gitMu.Unlock()
			fmt.Printf("   [Agent %d] 🔓 Git lock released (Sandbox spawned).\n", agentIdx)

			if err != nil {
				f.logger.ErrorContext(ctx, "failed to spawn sandbox", "error", err, "agent", agentIdx)
				results[agentIdx-1] = agentResult{skipped: true}
				return
			}

			// Ensure cleanup happens, also protected by mutex
			defer func() {
				gitMu.Lock()
				_ = svc.state.CloseSandbox(ctx, sandboxDir)
				gitMu.Unlock()
			}()

			targetDir := filepath.Join(sandboxDir, "chats", thread.ID, "docs")
			if err := os.MkdirAll(targetDir, 0755); err != nil {
				f.logger.ErrorContext(ctx, "failed to create thread docs directory", "error", err, "agent", agentIdx)
				results[agentIdx-1] = agentResult{skipped: true}
				return
			}

			maxRetries := 2
			success := false
			currentInstructions := fmt.Sprintf("%s\n\n%s", space.SubAgentSystemPrompt(), specificInstruction)

			// CONCURRENT: The LLM network call and domain verification run entirely in parallel
			for attempt := 1; attempt <= maxRetries; attempt++ {
				fmt.Printf("   [Agent %d] Writing code (Attempt %d/%d)...\n", agentIdx, attempt, maxRetries)

				if err := executor(ctx, currentInstructions, targetDir); err != nil {
					break
				}

				fmt.Printf("   [Agent %d] Verifying domain constraints...\n", agentIdx)
				if err := space.Verify(ctx, targetDir); err != nil {
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

			// Mutex: Prevent Git lock collisions when pushing branches back to the main repo
			gitMu.Lock()
			if _, err := svc.state.CommitSandbox(ctx, sandboxDir, commitMsg); err != nil {
				f.logger.ErrorContext(ctx, "failed to commit sandbox", "error", err, "agent", agentIdx)
				commitSuccess = false
			} else if err := svc.state.SubmitSandbox(ctx, sandboxDir, svc.workspaceRoot, candidateID); err != nil {
				f.logger.ErrorContext(ctx, "failed to submit sandbox", "error", err, "agent", agentIdx)
				commitSuccess = false
			}
			gitMu.Unlock()

			results[agentIdx-1] = agentResult{
				candidateID:   candidateID,
				branchName:    "candidate/" + candidateID,
				success:       success,
				commitSuccess: commitSuccess,
				skipped:       false,
			}
		}(i)
	}

	// Wait for all concurrent agents to finish
	wg.Wait()

	// Sequentially build the summary output so it remains ordered
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
