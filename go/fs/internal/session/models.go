package session

import (
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
)

type ManagerPayload struct {
	Branch   string
	Strategy string
}

func (p ManagerPayload) ToMap() map[string]any {
	m := make(map[string]any)
	if p.Branch != "" {
		m["branch"] = p.Branch
	}
	if p.Strategy != "" {
		m["strategy"] = p.Strategy
	}
	return m
}

type TurnContext struct {
	SpaceID             string
	ChatID              string
	PreviewID           string
	ManagerSystemPrompt string
	StickyDigests       []string
	RecentHistory       []string
	FileContexts        []FileContext
}

type FileContext struct {
	Path       string
	Level      string
	TokenCount int
}

type TransitionRequest struct {
	TargetDomain string
}

func (e *TransitionRequest) Error() string {
	return "domain transition requested to: " + e.TargetDomain
}

type ContextApprovalDecision struct {
	Approved               bool
	RetainedFilePaths      []string
	RetainedHistoryIndices []int
}

type UserInterface interface {
	OnTextChunk(text string)
	ReviewContext(ctxData TurnContext) ContextApprovalDecision
	ChooseNextStep() flows.FlowStrategy
	ReviewCandidate(branch string) (accepted bool)
	ConfirmTransition(targetDomain string, reasoning string) bool
}
