package session

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"google.golang.org/genai"
)

type UserInterface interface {
	OnTextChunk(text string)
	OnDelegationStart(agentCount int, instructions string)
	OnDelegationComplete(summary string)
	WantToReview() bool
	ReviewCandidate(branch string) (accepted bool)
}

type Coordinator struct {
	logger     *slog.Logger
	service    *workspace.Service
	llmMgr     *llm.Manager
	executor   workspace.SubAgentExecutor
	fanOutFlow workspace.Flow
}

func NewCoordinator(
	logger *slog.Logger,
	service *workspace.Service,
	llmMgr *llm.Manager,
	executor workspace.SubAgentExecutor,
	fanOutFlow workspace.Flow,
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
	managerModel := thinkSpace.Model(workspace.ModelCategoryManager)

	stream := c.llmMgr.GenerateStream(
		ctx,
		managerModel,
		thinkSpace.SystemPrompt(),
		thinkSpace.Tools(),
		history,
	)

	var fullModelResponse strings.Builder
	var interceptedTools []llm.ToolCall

	for chunk, err := range stream {
		if err != nil {
			return fmt.Errorf("stream failed: %w", err)
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
		if err := c.service.LogModelResponse(ctx, thread, fullModelResponse.String()); err != nil {
			c.logger.ErrorContext(ctx, "failed to log response", "error", err)
		}
	}

	for _, call := range interceptedTools {
		if call.Name == "propose_change" {
			countFloat, _ := call.Args["agent_count"].(float64)
			instructions, _ := call.Args["instructions"].(string)
			agentCount := int(countFloat)

			ui.OnDelegationStart(agentCount, instructions)

			result, err := c.fanOutFlow.Execute(ctx, c.service, thread, thinkSpace, call.Args, c.executor)
			if err != nil {
				c.logger.ErrorContext(ctx, "delegation flow failed", "error", err)
				continue
			}

			ui.OnDelegationComplete(result.Summary)
			if len(result.Branches) == 0 {
				continue
			}

			if !ui.WantToReview() {
				continue
			}

			// CORRECTED SEQUENCE: Check out FIRST, then Review
			for _, branch := range result.Branches {
				candidateID := strings.TrimPrefix(branch, "candidate/")

				// 1. Physically switch the Git working tree
				if err := c.service.PreviewCandidate(ctx, thread, candidateID); err != nil {
					c.logger.ErrorContext(ctx, "preview checkout failed", "error", err)
					continue
				}

				// 2. Now ask the user to look at it
				accept := ui.ReviewCandidate(branch)

				reason := "Rejected via triage"
				if accept {
					reason = "Accepted via triage"
				}

				if err := c.service.ResolveCandidate(ctx, thread, candidateID, accept, reason); err != nil {
					c.logger.ErrorContext(ctx, "failed to resolve candidate", "error", err)
				}

				if accept {
					break // Stop reviewing if they accepted one
				}
			}
		}
	}

	_, err := c.service.Checkpoint(ctx, thread, "End of session turn")
	return err
}
