package api

import "github.com/tinywideclouds.com/thinkspace/internal/session/flows"

// ProjectionCoordinate is the strict 3-tier routing address required by
// the frontend's async WebSocket cache. It maps directly to ProjectionKey.
type ProjectionCoordinate struct {
	SpaceID      string
	ChatID       string
	ProjectionID string
}

type SpaceState struct {
	ID           string
	Name         string
	IsConfigured bool
}

type InboundEvent struct {
	Type               string
	SubmitPrompt       *SubmitPrompt
	SelectStrategy     *SelectStrategy
	ReviewDecision     *ReviewDecision
	ApproveContext     *ApproveContext
	TransitionDecision *TransitionDecision
	LoadHistory        *LoadHistory
	LoadSubGraph       *LoadSubGraph
	ForkChat           *ForkChat
	InlineEdit         *InlineEdit
	SwitchBranch       *SwitchBranch
}

type SubmitPrompt struct {
	Text    string
	SpaceID string
	ChatID  string
}

type SelectStrategy struct {
	StrategyID flows.FlowStrategy
	AutoAccept bool
}

type ReviewDecision struct {
	Branch   string
	Accepted bool
	Reason   string
}

type ApproveContext struct {
	Approved               bool
	RetainedFilePaths      []string
	RetainedHistoryIndices []int
}

type TransitionDecision struct {
	Approved bool
}

type LoadHistory struct {
	SpaceID string
	ChatID  string
	Cursor  string
	Limit   int
}

type LoadSubGraph struct {
	SpaceID string
	ChatID  string
	GraphID string
}

type ForkChat struct {
	SpaceID       string
	ChatID        string
	TargetEventID string
}

type InlineEdit struct {
	SpaceID       string
	ChatID        string
	TargetEventID string
	NewText       string
}

type SwitchBranch struct {
	SpaceID      string
	ChatID       string
	TargetBranch string
}
