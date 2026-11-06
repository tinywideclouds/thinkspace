package session_test

import (
	"context"
	"iter"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type daTestModelClient struct {
	capturedBundle cognition.ThinkBundle
}

func (client *daTestModelClient) GenerateContentStream(
	ctx context.Context,
	model string,
	bundle cognition.ThinkBundle,
	registry *tools.Registry,
	config spaces.ThinkStateConfig,
) iter.Seq2[llm.StreamChunk, error] {
	client.capturedBundle = bundle
	return func(yield func(llm.StreamChunk, error) bool) {
		yield(llm.StreamChunk{Text: "Digest output"}, nil)
	}
}

type daTestChatEngine struct{}

func (e *daTestChatEngine) InitChat(ctx context.Context, chatID string) error { return nil }
func (e *daTestChatEngine) Snapshot(ctx context.Context, chatID, message string) (string, error) {
	return "", nil
}
func (e *daTestChatEngine) SpawnCandidateSandbox(ctx context.Context, chatID, candidateID string) (workspace.CandidateSandbox, error) {
	return nil, nil
}
func (e *daTestChatEngine) PreviewCandidate(ctx context.Context, chatID, candidateID string) error {
	return nil
}
func (e *daTestChatEngine) Accept(ctx context.Context, chatID, candidateID, reason string) error {
	return nil
}
func (e *daTestChatEngine) Reject(ctx context.Context, chatID, candidateID, reason string) error {
	return nil
}
func (e *daTestChatEngine) ReadCandidateDiff(ctx context.Context, chatID, candidateID string) (string, error) {
	return "", nil
}

func TestDigestAdapter_CommitDigest(t *testing.T) {
	client := &daTestModelClient{}
	cfg := session.DigestConfig{ManualPrompt: "Instruction: {{.Instruction}}\nContext: {{.Context}}"}
	manager := session.NewDigestManager(nil, client, cfg)

	// Create a minimal workspace service so it doesn't panic on nil dereference when saving
	svc := workspace.NewService(nil, &daTestChatEngine{}, t.TempDir(), thinkgraph.NewDispatcher(nil, nil), nil)
	adapter := session.NewDigestAdapter(manager, svc, "test-model", "Persona")

	address := &thinkgraph.ChatAddress{ChatID: "chat-1", ProjectID: "space-1"}

	_ = adapter.CommitDigest(context.Background(), address, "Topic", []string{"tag"}, "instruction data", "context data")

	catalystContent := client.capturedBundle.Trajectory.Catalyst.Content
	if catalystContent == "" {
		t.Errorf("Expected adapter to pass through to DigestManager and construct LLM prompt")
	}
}
