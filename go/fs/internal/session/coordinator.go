package session

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/tinywideclouds.com/thinkspace/internal/chat"
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
	ChooseNextStep() DelegationStrategy
	ReviewCandidate(branch string) (accepted bool)
}

type Coordinator struct {
	logger            *slog.Logger
	workspaceService  *workspace.Service
	llmAdapter        *llm.Adapter
	executor          workspace.SubAgentExecutor
	fanOutFlow        flows.Flow
	emitter           flows.FlowEmitter
	spaceID           string
	baseAgentRules    string
	flowConfiguration flows.FlowConfig
}

func NewCoordinator(
	logger *slog.Logger,
	workspaceService *workspace.Service,
	llmAdapter *llm.Adapter,
	executor workspace.SubAgentExecutor,
	fanOutFlow flows.Flow,
	emitter flows.FlowEmitter,
	spaceID string,
	baseAgentRules string,
	flowConfiguration flows.FlowConfig,
) *Coordinator {
	return &Coordinator{
		logger:            logger,
		workspaceService:  workspaceService,
		llmAdapter:        llmAdapter,
		executor:          executor,
		fanOutFlow:        fanOutFlow,
		emitter:           emitter,
		spaceID:           spaceID,
		baseAgentRules:    baseAgentRules,
		flowConfiguration: flowConfiguration,
	}
}

func (c *Coordinator) ExecuteTurn(
	ctx context.Context,
	thread *chat.Thread,
	thinkSpace workspace.ThinkSpace,
	history []*genai.Content,
	userInterface UserInterface,
) error {
	turnContext, cancel := context.WithTimeout(ctx, thinkSpace.TurnTimeout())
	defer cancel()

	managerModel := thinkSpace.Model(workspace.ModelCategoryManager)

	stream := c.llmAdapter.GenerateStream(
		turnContext,
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
					userInterface.OnTextChunk(part.Text)
					fullModelResponse.WriteString(part.Text)
				}
			}
		}

		calls := c.llmAdapter.InterceptToolCalls(chunk)
		interceptedTools = append(interceptedTools, calls...)
	}

	if fullModelResponse.Len() > 0 {
		if err := c.workspaceService.LogModelResponse(turnContext, thread, fullModelResponse.String()); err != nil {
			c.logger.ErrorContext(turnContext, "failed to log response", "error", err)
		}
		history = append(history, &genai.Content{
			Role:  "model",
			Parts: []*genai.Part{{Text: fullModelResponse.String()}},
		})
	}

	for _, call := range interceptedTools {
		if call.Name == "propose_change" {
			flowContext := flows.FlowContext{
				FlowID:         fmt.Sprintf("fanout-%s", thread.ID),
				SpaceID:        c.spaceID,
				BaseAgentRules: c.baseAgentRules,
			}

			result, err := c.fanOutFlow.Execute(
				turnContext,
				c.workspaceService,
				thread,
				thinkSpace,
				call.Args,
				c.flowConfiguration,
				flowContext,
				c.emitter,
				c.executor,
				thinkSpace.Verifier(),
			)

			if err != nil {
				c.logger.ErrorContext(turnContext, "delegation flow failed", "error", err)
				continue
			}

			if len(result.Branches) == 0 {
				continue
			}

			branchesToReview := result.Branches

			strategy := userInterface.ChooseNextStep()

			if strategy == StrategySkip {
				for _, branch := range branchesToReview {
					candidateID := strings.TrimPrefix(branch, "candidate/")
					_ = c.workspaceService.ResolveCandidate(turnContext, thread, candidateID, false, "Auto-rejected (Review Skipped)")
				}
				continue
			}

			if strategy == StrategyReview || strategy == StrategyRefine {
				branchesToReview = c.executeLLMReviewPhase(turnContext, thread, thinkSpace, history, userInterface, strategy, result.Branches)
			}

			hasAcceptedAny := false
			for _, branch := range branchesToReview {
				candidateID := strings.TrimPrefix(branch, "candidate/")

				if hasAcceptedAny {
					_ = c.workspaceService.ResolveCandidate(turnContext, thread, candidateID, false, "Auto-rejected (Another candidate was accepted)")
					continue
				}

				if err := c.workspaceService.PreviewCandidate(turnContext, thread, candidateID); err != nil {
					c.logger.ErrorContext(turnContext, "preview checkout failed", "error", err)
					_ = c.workspaceService.ResolveCandidate(turnContext, thread, candidateID, false, "Auto-rejected (Preview checkout failed)")
					continue
				}

				accept := userInterface.ReviewCandidate(branch)

				reason := "Rejected via triage"
				if accept {
					reason = "Accepted via triage"
					hasAcceptedAny = true
				}

				if err := c.workspaceService.ResolveCandidate(turnContext, thread, candidateID, accept, reason); err != nil {
					c.logger.ErrorContext(turnContext, "failed to resolve candidate", "error", err)
				}
			}
		}
	}

	_, err := c.workspaceService.Checkpoint(turnContext, thread, "End of session turn")
	return err
}

func (c *Coordinator) executeLLMReviewPhase(
	ctx context.Context,
	thread *chat.Thread,
	thinkSpace workspace.ThinkSpace,
	history []*genai.Content,
	userInterface UserInterface,
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

		diff, err := c.workspaceService.ReadCandidate(ctx, thread, candidateID)
		if err != nil || diff == "" {
			diff = "// No readable diff generated or error fetching diff."
		}

		diffsBuilder.WriteString(fmt.Sprintf("#### %s\n```diff\n%s\n```\n\n", branch, diff))
	}

	ledgerPrompt := ledgerPromptBuilder.String()
	llmPrompt := ledgerPrompt + "\n\n### Code Diffs\n\n" + diffsBuilder.String()

	if err := c.workspaceService.LogUserMessage(ctx, thread, ledgerPrompt); err != nil {
		c.logger.ErrorContext(ctx, "failed to log evaluation prompt to ledger", "error", err)
	}

	evaluationHistory := make([]*genai.Content, len(history))
	copy(evaluationHistory, history)
	evaluationHistory = append(evaluationHistory, &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: llmPrompt}},
	})

	userInterface.OnTextChunk("\n\n🤖 Manager evaluating candidates...\n")

	managerModel := thinkSpace.Model(workspace.ModelCategoryManager)
	evaluationStream := c.llmAdapter.GenerateStream(ctx, managerModel, thinkSpace.SystemPrompt(), nil, evaluationHistory)

	var evaluationResponse strings.Builder
	for chunk, err := range evaluationStream {
		if err != nil {
			c.logger.ErrorContext(ctx, "evaluation stream failed", "error", err)
			return candidateBranches
		}
		if len(chunk.Candidates) > 0 && chunk.Candidates[0].Content != nil {
			for _, part := range chunk.Candidates[0].Content.Parts {
				if part.Text != "" {
					userInterface.OnTextChunk(part.Text)
					evaluationResponse.WriteString(part.Text)
				}
			}
		}
	}

	if evaluationResponse.Len() > 0 {
		_ = c.workspaceService.LogModelResponse(ctx, thread, evaluationResponse.String())
	}

	if strategy == StrategyReview {
		return candidateBranches
	}

	userInterface.OnTextChunk("\n\n🚀 Synthesizing final candidate based on evaluation...\n")

	refinementArguments := map[string]any{
		"agent_count": float64(1),
		"agent_tasks": []any{
			map[string]any{
				"context_digest": "Synthesize a final implementation based on the following architectural evaluation.",
				"instruction":    evaluationResponse.String(),
			},
		},
	}

	flowContext := flows.FlowContext{
		FlowID:         fmt.Sprintf("refine-%s", thread.ID),
		SpaceID:        c.spaceID,
		BaseAgentRules: c.baseAgentRules,
	}

	refinementResult, err := c.fanOutFlow.Execute(
		ctx,
		c.workspaceService,
		thread,
		thinkSpace,
		refinementArguments,
		c.flowConfiguration,
		flowContext,
		c.emitter,
		c.executor,
		thinkSpace.Verifier(),
	)

	if err != nil {
		c.logger.ErrorContext(ctx, "refinement flow failed", "error", err)
		userInterface.OnTextChunk("\n\n⚠️ Refinement orchestration failed. Falling back to original candidates.\n")
		return candidateBranches
	}

	finalBranches := append(refinementResult.Branches, candidateBranches...)
	return finalBranches
}
