package session_test

import (
	"context"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dgraph-io/badger/v4"
	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/db"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

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

type dmMockModelClient struct {
	responses       []llm.StreamChunk
	capturedPrompts []string
}

func (c *dmMockModelClient) GenerateContentStream(
	ctx context.Context,
	model string,
	bundle cognition.ThinkBundle,
	registry *tools.Registry,
	config spaces.ThinkStateConfig,
) iter.Seq2[llm.StreamChunk, error] {
	c.capturedPrompts = append(c.capturedPrompts, bundle.Trajectory.Catalyst.Content)
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
	chatID := "test-digest-thread"

	ledger := thinkgraph.NewFileLedger(1000, workspaceRoot)
	dispatcher := thinkgraph.NewDispatcher(nil, ledger)
	emitter := &dmTestEmitter{}

	svc := workspace.NewService(logger, &dmMockChatEngine{}, workspaceRoot, dispatcher, emitter)

	chatAddress := &thinkgraph.ChatAddress{
		ChatID:    chatID,
		ProjectID: spaceID,
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}
	_ = svc.InitializeChat(context.Background(), chatAddress)

	client := &dmMockModelClient{
		responses: []llm.StreamChunk{{Text: "Manual summary content"}},
	}

	cfg := session.DigestConfig{ManualPrompt: "Instruction: {{.Instruction}}\nContext: {{.Context}}"}
	manager := session.NewDigestManager(logger, client, cfg)

	err := manager.CreateManualDigest(context.Background(), svc, chatAddress, "Topic A", []string{"tag1"}, false, "Summarize this", "Context data", "test-model", "Persona")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	digestsDir := filepath.Join(workspaceRoot, spaceID, "chats", chatAddress.ChatID, "ledger", "digests")
	entries, err := os.ReadDir(digestsDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected digest markdown file to be created in %s", digestsDir)
	}

	content, err := os.ReadFile(filepath.Join(digestsDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("failed to read digest file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "Manual summary content") {
		t.Errorf("expected content match in markdown file, got:\n%s", contentStr)
	}
	if !strings.Contains(contentStr, "sticky: false") {
		t.Errorf("expected sticky: false in markdown frontmatter, got:\n%s", contentStr)
	}
}

func TestDigestManager_CreateBackgroundDigest_ReusesExistingTags(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	workspaceRoot := t.TempDir()
	spaceID := "test-space"
	chatID := "test-bg-digest-thread"

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("failed to init badger: %v", err)
	}
	defer graphDB.Close()

	graphStore := thinkgraph.NewGraphStore(graphDB, nil)
	ledger := thinkgraph.NewFileLedger(1000, workspaceRoot)
	dispatcher := thinkgraph.NewDispatcher(graphStore, ledger)
	emitter := &dmTestEmitter{}

	svc := workspace.NewService(logger, &dmMockChatEngine{}, workspaceRoot, dispatcher, emitter)

	chatAddress := &thinkgraph.ChatAddress{
		ChatID:    chatID,
		ProjectID: spaceID,
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}
	if err := svc.InitializeChat(context.Background(), chatAddress); err != nil {
		t.Fatalf("InitializeChat failed: %v", err)
	}

	// Seed an existing digest with canonical tags
	if err := svc.AppendDigest(context.Background(), chatAddress, "Prior auth setup", []string{"auth", "http"}, false, nil); err != nil {
		t.Fatalf("AppendDigest failed: %v", err)
	}

	jsonResponse := `
	{
		"thought": "I will summarize",
		"digests": [
			{"topic_name": "Auth", "tags": ["Security", "#auth"], "summary": "Added login"}
		]
	}`

	client := &dmMockModelClient{
		responses: []llm.StreamChunk{{Text: jsonResponse}},
	}

	cfg := session.DigestConfig{BackgroundPrompt: "Tags: {{.ExistingTags}}\nContext: {{.TargetWindow}}"}
	manager := session.NewDigestManager(logger, client, cfg)

	ev1 := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Build auth")
	ev2 := thinkgraph.NewEvent(thinkgraph.EventModel, "Sure thing")
	targetWindow := []thinkgraph.Event{ev1, ev2}

	count, err := manager.CreateBackgroundDigest(context.Background(), svc, chatAddress, targetWindow, nil, false, "test-model", "Persona")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 digest created, got %d", count)
	}

	if len(client.capturedPrompts) == 0 || !strings.Contains(client.capturedPrompts[0], "auth, http") {
		t.Errorf("expected background digest prompt to include existing canonical tags 'auth, http', got: %v", client.capturedPrompts)
	}

	digestsDir := filepath.Join(workspaceRoot, spaceID, "chats", chatAddress.ChatID, "ledger", "digests")
	entries, err := os.ReadDir(digestsDir)
	if err != nil || len(entries) < 2 {
		t.Fatalf("expected 2 digest markdown files in %s", digestsDir)
	}

	foundAddedLogin := false
	for _, entry := range entries {
		content, readErr := os.ReadFile(filepath.Join(digestsDir, entry.Name()))
		if readErr != nil {
			t.Fatalf("failed to read digest file: %v", readErr)
		}
		contentStr := string(content)
		if strings.Contains(contentStr, "Added login") {
			foundAddedLogin = true
			if !strings.Contains(contentStr, "summarizes: [") {
				t.Errorf("expected summarizes array in frontmatter, got:\n%s", contentStr)
			}
			if !strings.Contains(contentStr, `tags: ["security", "auth"]`) {
				t.Errorf("expected normalized tags [\"security\", \"auth\"], got:\n%s", contentStr)
			}
		}
	}
	if !foundAddedLogin {
		t.Errorf("expected new digest containing 'Added login'")
	}
}
