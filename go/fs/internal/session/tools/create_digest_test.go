package tools_test

import (
	"context"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type mockUI struct {
	tools.UI
}

func (m *mockUI) OnTextChunk(chunk workspace.NodeChunk)                            {}
func (m *mockUI) ConfirmTransition(targetThinkState string, reasoning string) bool { return true }

type mockToolEmitter struct {
	events []telemetry.Event
}

func (m *mockToolEmitter) Emit(channel string, event telemetry.Event) {
	m.events = append(m.events, event)
}

type mockDigestCommitter struct {
	lastTopic       string
	lastTags        []string
	lastInstruction string
	lastContext     string
}

func (m *mockDigestCommitter) CommitDigest(ctx context.Context, address *thinkgraph.ChatAddress, topicName string, tags []string, instruction string, contextData string) error {
	m.lastTopic = topicName
	m.lastTags = tags
	m.lastInstruction = instruction
	m.lastContext = contextData
	return nil
}

func TestCreateDigestTool_Execute(t *testing.T) {
	committer := &mockDigestCommitter{}
	tool := tools.NewCreateDigestTool(committer)

	rawArgs := map[string]any{
		tools.TopicName:         "Architecture Decision",
		tools.DigestInstruction: "Summarize the DB choice",
		tools.Tags:              []any{"database", "architecture"},
	}

	bundle := &cognition.ThinkBundle{
		Trajectory: cognition.Trajectory{
			WorkingMemory: []cognition.TrajectoryEvent{
				{Role: "user", Content: "Let's use Postgres"},
				{Role: "model", Content: "Good choice."},
			},
		},
	}

	ec := tools.ExecutionContext{
		Address:      &thinkgraph.ChatAddress{ChatID: "chat-1", ProjectID: "space-1"},
		RawArguments: rawArgs,
		Bundle:       bundle,
		UI:           &mockUI{},
	}

	result, err := tool.Execute(context.Background(), ec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := result.Signal.(*signals.SignalTerminal); !ok {
		t.Errorf("expected tool to yield SignalTerminal, got %T", result.Signal)
	}

	if committer.lastTopic != "Architecture Decision" {
		t.Errorf("expected topic 'Architecture Decision', got %q", committer.lastTopic)
	}
	if committer.lastInstruction != "Summarize the DB choice" {
		t.Errorf("expected instruction 'Summarize the DB choice', got %q", committer.lastInstruction)
	}
	if len(committer.lastTags) != 2 || committer.lastTags[0] != "database" {
		t.Errorf("expected tags [database, architecture], got %v", committer.lastTags)
	}
	if committer.lastContext == "" {
		t.Errorf("expected context data to be built from working memory")
	}
}

func TestCreateDigestTool_InvalidSchema(t *testing.T) {
	committer := &mockDigestCommitter{}
	tool := tools.NewCreateDigestTool(committer)
	emitter := &mockToolEmitter{}

	rawArgs := map[string]any{
		"unknown_field": "data",
	}

	ec := tools.ExecutionContext{
		RawArguments: rawArgs,
		UI:           &mockUI{},
		Emitter:      emitter,
	}

	result, err := tool.Execute(context.Background(), ec)
	if err == nil {
		t.Fatalf("expected error on missing required fields")
	}
	if result.Signal != nil {
		t.Errorf("expected nil signal on error")
	}

	foundIntercept := false
	for _, e := range emitter.events {
		if e.Type == "system_intercept" {
			foundIntercept = true
		}
	}
	if !foundIntercept {
		t.Errorf("expected system_intercept telemetry event to be fired")
	}
}
