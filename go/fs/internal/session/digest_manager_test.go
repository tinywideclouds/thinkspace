package session_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

func TestDigestManager_CreateManualDigest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	workspaceRoot := t.TempDir()
	spaceID := "test-space"

	bus := thinkgraph.NewEventBus()
	bus.Subscribe(thinkgraph.NewLedgerSubscriber(1000, workspaceRoot))
	bus.Subscribe(thinkgraph.NewDigestSubscriber(workspaceRoot))

	svc := workspace.NewService(logger, &coordMockChatEngine{}, workspaceRoot, bus, nil)
	chatSession, _ := svc.InitializeChat(context.Background(), spaceID, "test-digest-thread")

	client := &coordMockModelClient{
		responses: []llm.StreamChunk{{Text: "Manual summary content"}},
	}

	cfg := session.DigestConfig{ManualPrompt: "Instruction: {{.Instruction}}\nContext: {{.Context}}"}
	manager := session.NewDigestManager(logger, client, cfg)

	err := manager.CreateManualDigest(context.Background(), svc, chatSession, "Topic A", []string{"tag1"}, "Summarize this", "Context data", "test-model", "Persona")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	digestsDir := filepath.Join(workspaceRoot, spaceID, "chats", chatSession.ID, "ledger", "digests")
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

	bus := thinkgraph.NewEventBus()
	bus.Subscribe(thinkgraph.NewLedgerSubscriber(1000, workspaceRoot))
	bus.Subscribe(thinkgraph.NewDigestSubscriber(workspaceRoot))

	svc := workspace.NewService(logger, &coordMockChatEngine{}, workspaceRoot, bus, nil)
	chatSession, _ := svc.InitializeChat(context.Background(), spaceID, "test-bg-digest-thread")

	jsonResponse := `
	{
		"thought": "I will summarize",
		"digests": [
			{"topic_name": "Auth", "tags": ["security"], "summary": "Added login"}
		]
	}`

	client := &coordMockModelClient{
		responses: []llm.StreamChunk{{Text: jsonResponse}},
	}

	cfg := session.DigestConfig{BackgroundPrompt: "Context: {{.TargetWindow}}"}
	manager := session.NewDigestManager(logger, client, cfg)

	targetWindow := []thinkgraph.Event{thinkgraph.NewEvent(thinkgraph.EventPrompt, "Build auth")}
	count, err := manager.CreateBackgroundDigest(context.Background(), svc, chatSession, targetWindow, nil, false, "test-model", "Persona")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 digest created, got %d", count)
	}

	digestsDir := filepath.Join(workspaceRoot, spaceID, "chats", chatSession.ID, "ledger", "digests")
	entries, err := os.ReadDir(digestsDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected digest markdown file to be created in %s", digestsDir)
	}

	content, err := os.ReadFile(filepath.Join(digestsDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("failed to read digest file: %v", err)
	}

	if !strings.Contains(string(content), "Added login") {
		t.Errorf("expected content match in markdown file, got:\n%s", string(content))
	}
}
