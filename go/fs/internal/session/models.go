package session

import (
	"context"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type TurnContext struct {
	SpaceID             string
	ChatID              string
	ResponseID          uuid.UUID
	PreviewID           string
	ManagerSystemPrompt string
	LongTermMemory      []string
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

// UserInterface is the pure contract required by the cognitive engines.
type UserInterface interface {
	tools.UI
	ReviewContext(turnContext TurnContext) ContextApprovalDecision
	ChooseNextStep() (flows.FlowStrategy, bool)
	ReviewCandidate(branch string) (accepted bool, reason string)

	GetChunkChannel() chan<- workspace.NodeChunk

	StreamGraphEvent(address *thinkgraph.ChatAddress, node thinkgraph.Node)
}

type ContextReviewPolicy interface {
	ShouldReview(settings workspace.SpaceSettings, responseID uuid.UUID, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus) bool
}

// TurnEngine represents a polymorphic execution block orchestrated by the ChatSession.
type TurnEngine interface {
	Execute(ctx context.Context, address *thinkgraph.ChatAddress, turnID uuid.UUID, catalystID uuid.UUID, signal signals.EngineSignal, ui UserInterface, emitter telemetry.Emitter) (signals.EngineSignal, error)
}
