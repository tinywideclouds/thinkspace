package session_test

import (
	"context"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// Renamed from coordMockChatEngine to avoid cross-test collisions
type dmMockChatEngine struct{}

func (m *dmMockChatEngine) InitChat(ctx context.Context, chatID string) error { return nil }
func (m *dmMockChatEngine) Snapshot(ctx context.Context, chatID string, message string) (string, error) {
	return "mock-sha", nil
}
func (m *dmMockChatEngine) SpawnCandidateSandbox(ctx context.Context, chatID string, candidateID string) (workspace.CandidateSandbox, error) {
	return nil, nil
}
func (m *dmMockChatEngine) PreviewCandidate(ctx context.Context, chatID string, candidateID string) error {
	return nil
}
func (m *dmMockChatEngine) Accept(ctx context.Context, chatID string, candidateID string, reason string) error {
	return nil
}
func (m *dmMockChatEngine) Reject(ctx context.Context, chatID string, candidateID string, reason string) error {
	return nil
}
func (m *dmMockChatEngine) ReadCandidateDiff(ctx context.Context, chatID, candidateID string) (string, error) {
	return "+ mock diff", nil
}

// Renamed from coordMockModelClient
type dmMockModelClient struct {
	responses []llm.StreamChunk
}

func (c *dmMockModelClient) GenerateContentStream(
	ctx context.Context,
	model string,
	bundle cognition.ThinkBundle,
	registry *tools.Registry,
	config spaces.ThinkStateConfig,
) iter.Seq2[llm.StreamChunk, error] {
	return func(yield func(llm.StreamChunk, error) bool) {
		for _, resp := range c.responses {
			yield(resp, nil)
		}
	}
}

type dmTestEmitter struct{}

func (emitter *dmTestEmitter) Emit(channel string, event telemetry.Event) {}

func TestDigestManager_CreateManualDigest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	workspaceRoot := t.TempDir()
	spaceID := "test-space"

	ledger := thinkgraph.NewFileLedger(1000, workspaceRoot)
	dispatcher := thinkgraph.NewDispatcher(nil, ledger)
	emitter := &dmTestEmitter{}

	svc := workspace.NewService(logger, &dmMockChatEngine{}, workspaceRoot, dispatcher, emitter)
	chatSession, _ := svc.InitializeChat(context.Background(), spaceID, "test-digest-thread")

	client := &dmMockModelClient{
		responses: []llm.StreamChunk{{Text: "Manual summary content"}},
	}

	cfg := session.DigestConfig{ManualPrompt: "Instruction: {{.Instruction}}\nContext: {{.Context}}"}
	manager := session.NewDigestManager(logger, client, cfg)

	err := manager.CreateManualDigest(context.Background(), svc, chatSession, "Topic A", []string{"tag1"}, "Summarize this", "Context data", "test-model", "Persona")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	digestsDir := filepath.Join(workspaceRoot, spaceID, "chats", chatSession.ChatID, "ledger", "digests")
	entries, err := os.ReadDir(digestsDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected digest markdown file to be created in %s", digestsDir)
	}

	content, err := os.ReadFile(filepath.Join(digestsDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("failed to read digest file: %v", err)
	}

	if !strings.Contains(string(content), "Manual summary content") {
		t.Errorf("expected content match in markdown file, got:\n%s", string(content))
	}
}

func TestDigestManager_CreateBackgroundDigest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	workspaceRoot := t.TempDir()
	spaceID := "test-space"

	ledger := thinkgraph.NewFileLedger(1000, workspaceRoot)
	dispatcher := thinkgraph.NewDispatcher(nil, ledger)
	emitter := &dmTestEmitter{}

	svc := workspace.NewService(logger, &dmMockChatEngine{}, workspaceRoot, dispatcher, emitter)
	chatSession, _ := svc.InitializeChat(context.Background(), spaceID, "test-bg-digest-thread")

	jsonResponse := `
	{
		"thought": "I will summarize",
		"digests": [
			{"topic_name": "Auth", "tags": ["security"], "summary": "Added login"}
		]
	}`

	client := &dmMockModelClient{
		responses: []llm.StreamChunk{{Text: jsonResponse}},
	}

	cfg := session.DigestConfig{BackgroundPrompt: "Context: {{.TargetWindow}}"}
	manager := session.NewDigestManager(logger, client, cfg)

	ev1 := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Build auth")
	ev2 := thinkgraph.NewEvent(thinkgraph.EventModel, "Sure thing")
	targetWindow := []thinkgraph.Event{ev1, ev2}

	count, err := manager.CreateBackgroundDigest(context.Background(), svc, chatSession, targetWindow, nil, false, "test-model", "Persona")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 digest created, got %d", count)
	}

	digestsDir := filepath.Join(workspaceRoot, spaceID, "chats", chatSession.ChatID, "ledger", "digests")
	entries, err := os.ReadDir(digestsDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected digest markdown file to be created in %s", digestsDir)
	}

	content, err := os.ReadFile(filepath.Join(digestsDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("failed to read digest file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "Added login") {
		t.Errorf("expected content match in markdown file, got:\n%s", contentStr)
	}

	if !strings.Contains(contentStr, "summarizes: [") {
		t.Errorf("expected summarizes array in frontmatter, got:\n%s", contentStr)
	}
	if !strings.Contains(contentStr, ev1.ID.String()) {
		t.Errorf("expected frontmatter to contain event ID %s", ev1.ID.String())
	}
	if !strings.Contains(contentStr, ev2.ID.String()) {
		t.Errorf("expected frontmatter to contain event ID %s", ev2.ID.String())
	}
}
