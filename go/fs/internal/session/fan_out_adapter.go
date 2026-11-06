package session

import (
	"context"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

// FanOutAdapter bridges the ProposeChangeTool to the FlowEngine,
// allowing any orchestrator to dispatch sub-agents without duplicating engine logic.
type FanOutAdapter struct {
	flowEngine *FlowEngine
	thinkState spaces.ThinkState
	spaceID    string
	ui         UserInterface
	fanOutFlow flows.Flow
}

func NewFanOutAdapter(
	flowEngine *FlowEngine,
	thinkState spaces.ThinkState,
	spaceID string,
	ui UserInterface,
	fanOutFlow flows.Flow,
) *FanOutAdapter {
	return &FanOutAdapter{
		flowEngine: flowEngine,
		thinkState: thinkState,
		spaceID:    spaceID,
		ui:         ui,
		fanOutFlow: fanOutFlow,
	}
}

func (f *FanOutAdapter) DispatchFanOut(ctx context.Context, address *thinkgraph.ChatAddress, bundle *cognition.ThinkBundle, args map[string]any, summary string) error {
	flowContext := flows.FlowContext{
		FlowID:         uuid.NewV7().String(),
		SpaceID:        f.spaceID,
		BaseAgentRules: f.thinkState.Config().WorkerSystemPrompt(),
		FlowSummary:    summary,
	}
	return f.flowEngine.Run(ctx, address, f.thinkState, f.fanOutFlow, args, flowContext, f.ui)
}
