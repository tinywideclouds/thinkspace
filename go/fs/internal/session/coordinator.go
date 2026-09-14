package session

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
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
	manifest *chat.ThreadManifest,
	thinkSpace spaces.ThinkSpace,
	history []*genai.Content,
	userInterface UserInterface,
) error {
	turnContext, cancel := context.WithTimeout(ctx, thinkSpace.Config().TurnTimeout())
	defer cancel()

	managerModel := thinkSpace.Config().Models[spaces.ModelCategoryManager]
	maxIterations := 3

	for iteration := 0; iteration < maxIterations; iteration++ {
		stream := c.llmAdapter.GenerateStream(
			turnContext,
			managerModel,
			thinkSpace.Config().ManagerSystemPrompt(),
			thinkSpace.Tools(),
			history,
		)

		var fullModelResponse strings.Builder
		var interceptedTools []llm.ToolCall

		for chunk, err := range stream {
			if err != nil {
				return fmt.Errorf("generation stream failed: %w", err)
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
			_ = c.workspaceService.LogModelResponse(turnContext, thread, fullModelResponse.String())
			history = append(history, &genai.Content{
				Role:  "model",
				Parts: []*genai.Part{{Text: fullModelResponse.String()}},
			})
		}

		if len(interceptedTools) == 0 {
			break
		}

		call := interceptedTools[0]

		if call.Name == "query_lens" {
			tag, _ := call.Args["tag"].(string)
			c.logger.InfoContext(turnContext, "executing query_lens", "tag", tag)
			userInterface.OnTextChunk(fmt.Sprintf("\n\n🔍 **Searching memory for #%s...**\n", tag))

			var toolResponse strings.Builder
			toolResponse.WriteString(fmt.Sprintf("[System Tool Response: query_lens(%s)]\n", tag))

			var idsToFetch []string
			if mappedUUIDs, exists := manifest.Lenses[tag]; exists {
				for _, u := range mappedUUIDs {
					idsToFetch = append(idsToFetch, u.String())
				}
			}

			if len(idsToFetch) == 0 {
				toolResponse.WriteString("No historical events found for this tag.\n")
			} else {
				events, err := c.workspaceService.FetchEvents(turnContext, thread, idsToFetch)
				if err != nil {
					toolResponse.WriteString(fmt.Sprintf("Error fetching events: %v\n", err))
				} else {
					for _, ev := range events {
						toolResponse.WriteString(fmt.Sprintf("- [%s] %s: %s\n", ev.Timestamp.Format("15:04:05"), ev.Type, ev.Content))
					}
				}
			}

			history = append(history, &genai.Content{
				Role:  "user",
				Parts: []*genai.Part{{Text: toolResponse.String()}},
			})
			continue
		}

		if call.Name == "propose_change" {
			err := c.executeProposeChange(turnContext, thread, thinkSpace, history, userInterface, call, managerModel)
			if err != nil {
				return err
			}
			break
		}
	}

	_, err := c.workspaceService.Checkpoint(turnContext, thread, "End of session turn")
	return err
}

func (c *Coordinator) executeProposeChange(
	turnContext context.Context,
	thread *chat.Thread,
	thinkSpace spaces.ThinkSpace,
	history []*genai.Content,
	userInterface UserInterface,
	call llm.ToolCall,
	managerModel string,
) error {
	var flowSummary strings.Builder
	uniqueFlowID := fmt.Sprintf("flow-%s", uuid.NewV7().String())

	flowContext := flows.FlowContext{
		FlowID:         uniqueFlowID,
		SpaceID:        c.spaceID,
		BaseAgentRules: c.baseAgentRules,
	}

	var assignedTags []string
	if rawTags, ok := call.Args["assigned_tags"].([]any); ok {
		for _, rt := range rawTags {
			if s, isStr := rt.(string); isStr {
				assignedTags = append(assignedTags, s)
			}
		}
	}

	activeFlowConfig := c.flowConfiguration
	if thinkSpace.Config().WorkerRetryPrompt != "" {
		activeFlowConfig.RetryPrompt = thinkSpace.Config().WorkerRetryPrompt
	}

	result, err := c.fanOutFlow.Execute(
		turnContext,
		c.workspaceService,
		thread,
		thinkSpace,
		call.Args,
		activeFlowConfig,
		flowContext,
		c.emitter,
		c.executor,
		thinkSpace.Verifier(),
	)

	if err != nil {
		c.logger.ErrorContext(turnContext, "delegation flow failed", "error", err)
		return err
	}

	if len(result.Branches) == 0 {
		return nil
	}

	for _, branch := range result.Branches {
		candidateID := strings.TrimPrefix(branch, "candidate/")
		_ = c.workspaceService.LogProposal(turnContext, thread, candidateID, fmt.Sprintf("Proposed branch %s via sub-agent fan-out.", branch), assignedTags)
	}

	branchesToReview := result.Branches
	strategy := userInterface.ChooseNextStep()

	strategyNames := map[DelegationStrategy]string{
		StrategySkip:   "Skip / Abort",
		StrategyManual: "Manual Review",
		StrategyReview: "Assisted Review",
		StrategyRefine: "Refine (Manager synthesized final version)",
	}

	_ = c.workspaceService.LogUserMessage(turnContext, thread, fmt.Sprintf("[User selected triage strategy: %s]", strategyNames[strategy]))
	flowSummary.WriteString(fmt.Sprintf("- User selected triage strategy: %s\n", strategyNames[strategy]))

	if strategy == StrategySkip {
		for _, branch := range branchesToReview {
			candidateID := strings.TrimPrefix(branch, "candidate/")
			_ = c.workspaceService.ResolveCandidate(turnContext, thread, candidateID, false, "Auto-rejected (Review Skipped)")
			flowSummary.WriteString(fmt.Sprintf("- Candidate %s was Auto-Rejected.\n", candidateID))
		}
	} else {
		if strategy == StrategyReview || strategy == StrategyRefine {
			branchesToReview = c.executeLLMReviewPhase(turnContext, thread, thinkSpace, history, userInterface, strategy, result.Branches)
		}

		hasAcceptedAny := false
		for _, branch := range branchesToReview {
			candidateID := strings.TrimPrefix(branch, "candidate/")

			if hasAcceptedAny {
				_ = c.workspaceService.ResolveCandidate(turnContext, thread, candidateID, false, "Auto-rejected (Another candidate was accepted)")
				flowSummary.WriteString(fmt.Sprintf("- Candidate %s was Auto-Rejected (conflict).\n", candidateID))
				continue
			}

			if err := c.workspaceService.PreviewCandidate(turnContext, thread, candidateID); err != nil {
				_ = c.workspaceService.ResolveCandidate(turnContext, thread, candidateID, false, "Auto-rejected (Preview checkout failed)")
				flowSummary.WriteString(fmt.Sprintf("- Candidate %s was Auto-Rejected (checkout failed).\n", candidateID))
				continue
			}

			accept := userInterface.ReviewCandidate(branch)
			reason, status := "Rejected via triage", "REJECTED"
			if accept {
				reason, status = "Accepted via triage", "ACCEPTED"
				hasAcceptedAny = true
			}

			flowSummary.WriteString(fmt.Sprintf("- Candidate %s was %s by the user.\n", candidateID, status))
			_ = c.workspaceService.ResolveCandidate(turnContext, thread, candidateID, accept, reason)
		}
	}

	// Wrap Up
	userInterface.OnTextChunk("\n\n🤖 **Manager summarizing turn...**\n")
	wrapUpPrompt := fmt.Sprintf(
		"The sub-agent orchestration flow is now complete. Here is the system trace of the outcome:\n\n%s\n\n"+
			"Please provide a friendly, conversational wrap-up to the human user.", flowSummary.String(),
	)

	history = append(history, &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: wrapUpPrompt}},
	})

	wrapUpStream := c.llmAdapter.GenerateStream(turnContext, managerModel, thinkSpace.Config().ManagerSystemPrompt(), nil, history)
	var wrapUpResponse strings.Builder
	for chunk, _ := range wrapUpStream {
		if len(chunk.Candidates) > 0 && chunk.Candidates[0].Content != nil {
			for _, part := range chunk.Candidates[0].Content.Parts {
				if part.Text != "" {
					userInterface.OnTextChunk(part.Text)
					wrapUpResponse.WriteString(part.Text)
				}
			}
		}
	}

	if wrapUpResponse.Len() > 0 {
		_ = c.workspaceService.LogModelResponse(turnContext, thread, wrapUpResponse.String())
	}
	return nil
}

func (c *Coordinator) executeLLMReviewPhase(
	ctx context.Context,
	thread *chat.Thread,
	thinkSpace spaces.ThinkSpace,
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

	userInterface.OnTextChunk("\n\n🤖 **Manager evaluating candidates...**\n")

	managerModel := thinkSpace.Config().Models[spaces.ModelCategoryManager]
	evaluationStream := c.llmAdapter.GenerateStream(ctx, managerModel, thinkSpace.Config().ManagerSystemPrompt(), nil, evaluationHistory)

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

	userInterface.OnTextChunk("\n\n🚀 **Synthesizing final candidate based on evaluation...**\n")

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

	activeFlowConfig := c.flowConfiguration
	if thinkSpace.Config().WorkerRetryPrompt != "" {
		activeFlowConfig.RetryPrompt = thinkSpace.Config().WorkerRetryPrompt
	}

	refinementResult, err := c.fanOutFlow.Execute(
		ctx,
		c.workspaceService,
		thread,
		thinkSpace,
		refinementArguments,
		activeFlowConfig,
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
