package workspace

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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

	// RESTORED: Log the exact instructions being sent to each agent
	for i, inst := range rawInstructions {
		f.logger.InfoContext(ctx, "sub-agent instruction payload",
			slog.Int("agent_index", i+1),
			slog.String("instruction", fmt.Sprintf("%v", inst)),
		)
	}

	var branches []string
	var summaryBuilder strings.Builder
	fmt.Fprintf(&summaryBuilder, "Delegation Results (%d Agents):\n\n", count)

	baseID := fmt.Sprintf("proposal-%d", time.Now().Unix())

	for i := 1; i <= count; i++ {
		candidateID := fmt.Sprintf("%s-%d", baseID, i)

		specificInstruction := "Implement the requested feature."
		if i-1 < len(rawInstructions) {
			specificInstruction = rawInstructions[i-1].(string)
		}

		fmt.Printf("   [Agent %d] Spawning sandbox...\n", i)

		sandboxDir, err := svc.state.SpawnSandbox(ctx, svc.workspaceRoot, thread.ID, candidateID)
		if err != nil {
			f.logger.ErrorContext(ctx, "failed to spawn sandbox", "error", err)
			continue
		}

		targetDir := filepath.Join(sandboxDir, "chats", thread.ID, "docs")
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			f.logger.ErrorContext(ctx, "failed to create thread docs directory", "error", err)
			_ = svc.state.CloseSandbox(ctx, sandboxDir)
			continue
		}

		maxRetries := 2
		success := false
		currentInstructions := fmt.Sprintf("%s\n\n%s", space.SubAgentSystemPrompt(), specificInstruction)

		for attempt := 1; attempt <= maxRetries; attempt++ {
			fmt.Printf("   [Agent %d] Writing code (Attempt %d/%d)...\n", i, attempt, maxRetries)

			if err := executor(ctx, currentInstructions, targetDir); err != nil {
				break
			}

			fmt.Printf("   [Agent %d] Verifying domain constraints...\n", i)
			if err := space.Verify(ctx, targetDir); err != nil {
				fmt.Printf("   [Agent %d] ⚠️ Verification failed: %v\n", i, err)
				currentInstructions = fmt.Sprintf("%s\n\nVerification failed:\n%s\nPlease fix.", specificInstruction, err.Error())
				continue
			}

			success = true
			fmt.Printf("   [Agent %d] ✅ Verification passed.\n", i)
			break
		}

		// ALWAYS commit and push the sandbox, even if domain verification failed.
		commitMsg := fmt.Sprintf("auto(candidate): proposal %s", candidateID)
		commitSuccess := true

		if _, err := svc.state.CommitSandbox(ctx, sandboxDir, commitMsg); err != nil {
			f.logger.ErrorContext(ctx, "failed to commit sandbox", "error", err)
			commitSuccess = false
		} else if err := svc.state.SubmitSandbox(ctx, sandboxDir, svc.workspaceRoot, candidateID); err != nil {
			f.logger.ErrorContext(ctx, "failed to submit sandbox", "error", err)
			commitSuccess = false
		}

		if commitSuccess {
			branchName := "candidate/" + candidateID
			branches = append(branches, branchName)
			if success {
				fmt.Fprintf(&summaryBuilder, "- %s (Verified: true)\n", branchName)
			} else {
				fmt.Fprintf(&summaryBuilder, "- %s (Verified: false - Failed Domain Constraints)\n", branchName)
			}
		} else {
			fmt.Fprintf(&summaryBuilder, "- %s (Git Submission Failed)\n", candidateID)
		}

		_ = svc.state.CloseSandbox(ctx, sandboxDir)
	}

	return &FlowResult{Branches: branches, Summary: summaryBuilder.String()}, nil
}
