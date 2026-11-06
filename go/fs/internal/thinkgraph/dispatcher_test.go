package thinkgraph_test

import (
	"context"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

type mockEventWriter struct {
	appendCount int
}

func (m *mockEventWriter) AppendEvent(ctx context.Context, c *thinkgraph.ChatAddress, event thinkgraph.Event) error {
	m.appendCount++
	return nil
}
func (m *mockEventWriter) SaveDigest(ctx context.Context, c *thinkgraph.ChatAddress, digest thinkgraph.DigestMeta) error {
	return nil
}
func (m *mockEventWriter) SaveSpaceContext(ctx context.Context, c *thinkgraph.ChatAddress, spaceCtx thinkgraph.SpaceContext) error {
	return nil
}
func (m *mockEventWriter) SaveMutation(ctx context.Context, c *thinkgraph.ChatAddress, mutation thinkgraph.MutationEvent) error {
	return nil
}

func TestDispatcher_NilSafety(t *testing.T) {
	ctx := context.Background()
	chat := &thinkgraph.ChatAddress{ChatID: "chat-1", ProjectID: "space-1"}
	event := thinkgraph.NewEvent(thinkgraph.EventPrompt, "hello")

	// Test with completely nil sinks (should warn but not panic)
	dispatcher := thinkgraph.NewDispatcher(nil, nil)

	err := dispatcher.AppendEvent(ctx, chat, event)
	if err != nil {
		t.Errorf("expected no error when appending to nil sinks, got %v", err)
	}

	// Test with only Ledger configured
	ledger := &mockEventWriter{}
	dispatcherWithLedger := thinkgraph.NewDispatcher(nil, ledger)

	_ = dispatcherWithLedger.AppendEvent(ctx, chat, event)
	if ledger.appendCount != 1 {
		t.Errorf("expected ledger append count 1, got %d", ledger.appendCount)
	}
}
