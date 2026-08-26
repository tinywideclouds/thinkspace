package session

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace/flows"
	"google.golang.org/genai"
)

type DelegationStrategy int

const (
	StrategySkip DelegationStrategy = iota
	StrategyManual
	StrategyReview
	StrategyRefine
)

type UserInterface interface {
	OnTextChunk(text string)
	OnDelegationStart(agentCount int, instructions string)
	OnDelegationComplete(summary string)
	ChooseNextStep() DelegationStrategy
	ReviewCandidate(branch string) (accepted bool)
	GetAgentTokenChannel() chan<- workspace.AgentToken
}

type Coordinator struct {
	logger     *slog.Logger
	service    *workspace.Service
	llmMgr     *llm.Manager
	executor   workspace.SubAgentExecutor
	fanOutFlow flows.Flow
}

func NewCoordinator(
	logger *slog.Logger,
	service *workspace.Service,
	llmMgr *llm.Manager,
	executor workspace.SubAgentExecutor,
	fanOutFlow flows.Flow,
) *Coordinator {
	return &Coordinator{
		logger:     logger,
		service:    service,
		llmMgr:     llmMgr,
		executor:   executor,
		fanOutFlow: fanOutFlow,
	}
}

func (c *Coordinator) ExecuteTurn(
	ctx context.Context,
	thread *workspace.Thread,
	thinkSpace workspace.ThinkSpace,
	history []*genai.Content,
	ui UserInterface,
) error {
	turnCtx, cancel := context.WithTimeout(ctx, thinkSpace.TurnTimeout())
	defer cancel()

	managerModel := thinkSpace.Model(workspace.ModelCategoryManager)

	stream := c.llmMgr.GenerateStream(
		turnCtx,
		managerModel,
		thinkSpace.SystemPrompt(),
		thinkSpace.Tools(),
		history,
	)

	var fullModelResponse strings.Builder
	var interceptedTools []llm.ToolCall

	for chunk, err := range stream {
		if err != nil {
			return fmt.Errorf("initial stream failed: %w", err)
		}

		if len(chunk.Candidates) > 0 && chunk.Candidates[0].Content != nil {
			for _, part := range chunk.Candidates[0].Content.Parts {
				if part.Text != "" {
					ui.OnTextChunk(part.Text)
					fullModelResponse.WriteString(part.Text)
				}
			}
		}

		calls := c.llmMgr.InterceptToolCalls(chunk)
		interceptedTools = append(interceptedTools, calls...)
	}

	if fullModelResponse.Len() > 0 {
		if err := c.service.LogModelResponse(turnCtx, thread, fullModelResponse.String()); err != nil {
			c.logger.ErrorContext(turnCtx, "failed to log response", "error", err)
		}
		history = append(history, &genai.Content{
			Role:  "model",
			Parts: []*genai.Part{{Text: fullModelResponse.String()}},
		})
	}

	for _, call := range interceptedTools {
		if call.Name == "propose_change" {
			countFloat, _ := call.Args["agent_count"].(float64)
			agentCount := int(countFloat)

			instructionStr := fmt.Sprintf("Spawning %d agents to propose implementations.", agentCount)
			ui.OnDelegationStart(agentCount, instructionStr)

			result, err := c.fanOutFlow.Execute(turnCtx, c.service, thread, thinkSpace, call.Args, c.executor, ui.GetAgentTokenChannel())
			if err != nil {
				c.logger.ErrorContext(turnCtx, "delegation flow failed", "error", err)
				continue
			}

			ui.OnDelegationComplete(result.Summary)
			if len(result.Branches) == 0 {
				continue
			}

			branchesToReview := result.Branches

			strategy := ui.ChooseNextStep()

			if strategy == StrategySkip {
				for _, branch := range branchesToReview {
					candidateID := strings.TrimPrefix(branch, "candidate/")
					_ = c.service.ResolveCandidate(turnCtx, thread, candidateID, false, "Auto-rejected (Review Skipped)")
				}
				continue
			}

			if strategy == StrategyReview || strategy == StrategyRefine {
				branchesToReview = c.executeLLMReviewPhase(turnCtx, thread, thinkSpace, history, ui, strategy, result.Branches)
			}

			hasAcceptedAny := false
			for _, branch := range branchesToReview {
				candidateID := strings.TrimPrefix(branch, "candidate/")

				if hasAcceptedAny {
					_ = c.service.ResolveCandidate(turnCtx, thread, candidateID, false, "Auto-rejected (Another candidate was accepted)")
					continue
				}

				if err := c.service.PreviewCandidate(turnCtx, thread, candidateID); err != nil {
					c.logger.ErrorContext(turnCtx, "preview checkout failed", "error", err)
					_ = c.service.ResolveCandidate(turnCtx, thread, candidateID, false, "Auto-rejected (Preview checkout failed)")
					continue
				}

				accept := ui.ReviewCandidate(branch)

				reason := "Rejected via triage"
				if accept {
					reason = "Accepted via triage"
					hasAcceptedAny = true
				}

				if err := c.service.ResolveCandidate(turnCtx, thread, candidateID, accept, reason); err != nil {
					c.logger.ErrorContext(turnCtx, "failed to resolve candidate", "error", err)
				}
			}
		}
	}

	_, err := c.service.Checkpoint(turnCtx, thread, "End of session turn")
	return err
}

func (c *Coordinator) executeLLMReviewPhase(
	ctx context.Context,
	thread *workspace.Thread,
	thinkSpace workspace.ThinkSpace,
	history []*genai.Content,
	ui UserInterface,
	strategy DelegationStrategy,
	candidateBranches []string,
) []string {
	var ledgerPromptBuilder strings.Builder
	ledgerPromptBuilder.WriteString("The automated sub-agents have completed their proposals. Please evaluate the following candidate diffs. ")

	if strategy == StrategyRefine {
		ledgerPromptBuilder.WriteString("Write a detailed comparative summary. Based on your evaluation, your entire response will then be used as the architectural instruction for a single, final sub-agent to synthesize the ultimate best version.\n\n")
	} else {
		ledgerPromptBuilder.WriteString("Write a comparative summary and recommend the best approach to the human user.\n\n")
	}

	ledgerPromptBuilder.WriteString("### Candidate Branches\n\n")

	var diffsBuilder strings.Builder
	for _, branch := range candidateBranches {
		candidateID := strings.TrimPrefix(branch, "candidate/")

		ledgerPromptBuilder.WriteString(fmt.Sprintf("* [`%s`](#branch:%s)\n", branch, branch))

		diff, err := c.service.ReadCandidate(ctx, thread, candidateID)
		if err != nil || diff == "" {
			diff = "// No readable diff generated or error fetching diff."
		}

		diffsBuilder.WriteString(fmt.Sprintf("#### %s\n```diff\n%s\n```\n\n", branch, diff))
	}

	ledgerPrompt := ledgerPromptBuilder.String()
	llmPrompt := ledgerPrompt + "\n\n### Code Diffs\n\n" + diffsBuilder.String()

	if err := c.service.LogUserMessage(ctx, thread, ledgerPrompt); err != nil {
		c.logger.ErrorContext(ctx, "failed to log evaluation prompt to ledger", "error", err)
	}

	evalHistory := make([]*genai.Content, len(history))
	copy(evalHistory, history)
	evalHistory = append(evalHistory, &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: llmPrompt}},
	})

	ui.OnTextChunk("\n\n🤖 Manager evaluating candidates...\n")

	managerModel := thinkSpace.Model(workspace.ModelCategoryManager)
	evalStream := c.llmMgr.GenerateStream(ctx, managerModel, thinkSpace.SystemPrompt(), nil, evalHistory)

	var evaluationResponse strings.Builder
	for chunk, err := range evalStream {
		if err != nil {
			c.logger.ErrorContext(ctx, "evaluation stream failed", "error", err)
			return candidateBranches
		}
		if len(chunk.Candidates) > 0 && chunk.Candidates[0].Content != nil {
			for _, part := range chunk.Candidates[0].Content.Parts {
				if part.Text != "" {
					ui.OnTextChunk(part.Text)
					evaluationResponse.WriteString(part.Text)
				}
			}
		}
	}

	if evaluationResponse.Len() > 0 {
		_ = c.service.LogModelResponse(ctx, thread, evaluationResponse.String())
	}

	if strategy == StrategyReview {
		return candidateBranches
	}

	ui.OnTextChunk("\n\n🚀 Synthesizing final candidate based on evaluation...\n")

	refineArgs := map[string]any{
		"agent_count": float64(1),
		"agent_instructions": []any{
			fmt.Sprintf("Synthesize a final implementation based on this architectural evaluation:\n\n%s", evaluationResponse.String()),
		},
	}

	refineResult, err := c.fanOutFlow.Execute(ctx, c.service, thread, thinkSpace, refineArgs, c.executor, ui.GetAgentTokenChannel())
	if err != nil {
		c.logger.ErrorContext(ctx, "refinement flow failed", "error", err)
		ui.OnTextChunk("\n\n⚠️ Refinement orchestration failed. Falling back to original candidates.\n")
		return candidateBranches
	}

	ui.OnDelegationComplete(refineResult.Summary)

	finalBranches := append(refineResult.Branches, candidateBranches...)
	return finalBranches
}
