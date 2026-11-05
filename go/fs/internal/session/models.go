package session

import (
	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type TurnContext struct {
	SpaceID             string
	ChatID              string
	PreviewID           string
	ManagerSystemPrompt string
	StickyDigests       []string
	RecentHistory       []string
	FileContexts        []FileContext
	Focus               thinkgraph.AttentionFocus
}

type FileContext struct {
	Path       string
	Level      string
	TokenCount int
}

type ContextApprovalDecision struct {
	Approved               bool
	RetainedFilePaths      []string
	RetainedHistoryIndices []int
}

type UserInterface interface {
	OnTextChunk(text string)
	ReviewContext(turnContext TurnContext) ContextApprovalDecision
	ChooseNextStep() flows.FlowStrategy
	ReviewCandidate(branch string) (accepted bool)
	ConfirmTransition(targetThinkState string, reasoning string) bool
	GetAgentTokenChannel() chan<- workspace.AgentToken
}

// ContextReviewPolicy determines if a human-in-the-loop context review is required.
type ContextReviewPolicy interface {
	ShouldReview(settings workspace.SpaceSettings, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus) bool
}
