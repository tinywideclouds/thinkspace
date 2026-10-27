package api

import "github.com/tinywideclouds.com/thinkspace/internal/session/flows"

type SpaceState struct {
	ID           string
	Name         string
	IsConfigured bool
}

type InboundEvent struct {
	Type               string
	SubmitPrompt       *SubmitPromptPayload
	SelectStrategy     *SelectStrategyPayload
	ReviewDecision     *ReviewDecisionPayload
	ApproveContext     *ApproveContextPayload
	TransitionDecision *TransitionDecisionPayload
	LoadHistory        *LoadHistoryPayload
	ForkChat           *ForkChatPayload
	InlineEdit         *InlineEditPayload
	SwitchBranch       *SwitchBranchPayload
}

type SubmitPromptPayload struct {
	Text    string
	SpaceID string
	ChatID  string
}

type SelectStrategyPayload struct {
	StrategyID flows.FlowStrategy
}

type ReviewDecisionPayload struct {
	Branch   string
	Accepted bool
}

type ApproveContextPayload struct {
	Approved               bool
	RetainedFilePaths      []string
	RetainedHistoryIndices []int
}

type TransitionDecisionPayload struct {
	Approved bool
}

type LoadHistoryPayload struct {
	SpaceID string
	ChatID  string
	Cursor  string
	Limit   int
}

type ForkChatPayload struct {
	SpaceID       string
	ChatID        string
	TargetEventID string
}

type InlineEditPayload struct {
	SpaceID       string
	ChatID        string
	TargetEventID string
	NewText       string
}

type SwitchBranchPayload struct {
	SpaceID      string
	ChatID       string
	TargetBranch string
}
