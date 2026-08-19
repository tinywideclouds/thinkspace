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

// FanOutFlow implements the Flow interface for parallel, multi-agent code generation.
type FanOutFlow struct {
	logger *slog.Logger
}

func NewFanOutFlow(logger *slog.Logger) *FanOutFlow {
	return &FanOutFlow{logger: logger}
}

// Name now maps to our unified tool name.
func (f *FanOutFlow) Name() string {
	return "propose_change"
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

		// ALIGNMENT FIX: Target the specific thread's docs folder
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

			// Pass targetDir so files write safely inside the chat context
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

		if success {
			commitMsg := fmt.Sprintf("auto(candidate): proposal %s", candidateID)
			if _, err := svc.state.CommitSandbox(ctx, sandboxDir, commitMsg); err != nil {
				f.logger.ErrorContext(ctx, "failed to commit sandbox", "error", err)
				success = false
			} else if err := svc.state.SubmitSandbox(ctx, sandboxDir, svc.workspaceRoot, candidateID); err != nil {
				f.logger.ErrorContext(ctx, "failed to submit sandbox", "error", err)
				success = false
			}
		}

		if success {
			branchName := "candidate/" + candidateID
			branches = append(branches, branchName)
			fmt.Fprintf(&summaryBuilder, "- %s (Verified: %t)\n", branchName, success)
		} else {
			fmt.Fprintf(&summaryBuilder, "- %s (Failed)\n", candidateID)
		}

		_ = svc.state.CloseSandbox(ctx, sandboxDir)
	}

	return &FlowResult{Branches: branches, Summary: summaryBuilder.String()}, nil
}
