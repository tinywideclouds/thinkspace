package session_test

import (
	"context"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type foTestFlow struct {
	nextCalled bool
}

func (f *foTestFlow) Name() string  { return "TestFlow" }
func (f *foTestFlow) MaxSteps() int { return 10 }
func (f *foTestFlow) Next(state flows.FlowState) flows.FlowDirective {
	f.nextCalled = true
	return flows.FlowDirective{Action: flows.ActionFinish}
}

type foTestThinkSpace struct{}

func (s *foTestThinkSpace) Config() spaces.ThinkStateConfig                        { return spaces.ThinkStateConfig{} }
func (s *foTestThinkSpace) Verifier() spaces.Verifier                              { return nil }
func (s *foTestThinkSpace) Mapbook() spaces.Mapbook                                { return nil }
func (s *foTestThinkSpace) Patcher() workspace.Patcher                             { return nil }
func (s *foTestThinkSpace) Initialize(ctx context.Context, directory string) error { return nil }
func (s *foTestThinkSpace) EnrichBundle(ctx context.Context, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus, workspaceRoot string) error {
	return nil
}

func TestFanOutAdapter_DispatchFanOut(t *testing.T) {
	flow := &foTestFlow{}
	flowEngine := session.NewFlowEngine(nil, nil, nil, nil, nil) // Mock engine dependencies as it will just call flow.Next()
	adapter := session.NewFanOutAdapter(flowEngine, &foTestThinkSpace{}, "space-1", nil, flow)

	address := &thinkgraph.ChatAddress{ChatID: "chat-1", ProjectID: "space-1"}
	args := map[string]any{"agent_tasks": []any{}}

	// A real dispatch will call FlowEngine.Run, which queries flow.Next() immediately if the steps aren't exhausted.
	_ = adapter.DispatchFanOut(context.Background(), address, &cognition.ThinkBundle{}, args, "Summary")

	if !flow.nextCalled {
		t.Errorf("Expected adapter to successfully bridge execution to the FlowEngine and evaluate the Flow")
	}
}
