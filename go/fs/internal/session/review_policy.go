package session

import (
	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// TokenThresholdPolicy is the default heuristic for context review.
type TokenThresholdPolicy struct{}

func NewTokenThresholdPolicy() *TokenThresholdPolicy {
	return &TokenThresholdPolicy{}
}

func (policy *TokenThresholdPolicy) ShouldReview(settings workspace.SpaceSettings, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus) bool {
	if !settings.ReviewPolicy.Active {
		return false
	}

	turnContext := ProjectTurnContext("", "", bundle, focus)
	estimatedTokens := policy.estimateTokens(turnContext, settings.ReviewPolicy.IncludePersona)

	return estimatedTokens >= settings.ReviewPolicy.MinTokens
}

func (policy *TokenThresholdPolicy) estimateTokens(turnContext TurnContext, includePersona bool) int {
	var totalCharacters int

	if includePersona {
		totalCharacters += len(turnContext.ManagerSystemPrompt)
	}

	for _, digest := range turnContext.StickyDigests {
		totalCharacters += len(digest)
	}

	for _, history := range turnContext.RecentHistory {
		totalCharacters += len(history)
	}

	// Heuristic approximation: 1 token ~= 4 characters for raw text
	estimatedTextTokens := totalCharacters / 4

	fileTokens := 0
	for _, fileContext := range turnContext.FileContexts {
		fileTokens += fileContext.TokenCount
	}

	return estimatedTextTokens + fileTokens
}
