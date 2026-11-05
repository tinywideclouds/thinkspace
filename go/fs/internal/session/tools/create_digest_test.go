package tools_test

import (
	"context"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

type mockUI struct {
	tools.UI
}

func (m *mockUI) OnTextChunk(text string)                                   {}
func (m *mockUI) ConfirmTransition(targetThinkState, reasoning string) bool { return true }

type mockDigestCommitter struct {
	lastTopic       string
	lastTags        []string
	lastInstruction string
	lastContext     string
}

func (m *mockDigestCommitter) CommitDigest(ctx context.Context, chatObj *thinkgraph.Chat, topicName string, tags []string, instruction string, contextData string) error {
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
		ChatID:       "chat-1",
		SpaceID:      "space-1",
		RawArguments: rawArgs,
		Bundle:       bundle,
		UI:           &mockUI{},
	}

	shouldBreak, err := tool.Execute(context.Background(), ec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !shouldBreak {
		t.Errorf("expected tool to break the loop")
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

	rawArgs := map[string]any{
		"unknown_field": "data",
	}

	ec := tools.ExecutionContext{
		RawArguments: rawArgs,
		UI:           &mockUI{},
	}

	shouldBreak, err := tool.Execute(context.Background(), ec)
	if err == nil {
		t.Fatalf("expected error on missing required fields")
	}
	if !shouldBreak {
		t.Errorf("expected tool to break the loop to force LLM self-correction")
	}
}
