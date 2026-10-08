package session_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

func TestDigestManager_CreateManualDigest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	workspaceRoot := t.TempDir()
	bus := chat.NewEventBus()
	bus.Subscribe(chat.NewLedgerSubscriber())

	svc := workspace.NewService(logger, &coordMockChatEngine{}, workspaceRoot, bus)
	thread, _ := svc.StartThread(context.Background(), "test-digest-thread")

	client := &coordMockModelClient{
		responses: []llm.StreamChunk{{Text: "Manual summary content"}},
	}

	cfg := session.DigestConfig{ManualPrompt: "Instruction: {{.Instruction}}\nContext: {{.Context}}"}
	manager := session.NewDigestManager(logger, client, cfg)

	err := manager.CreateManualDigest(context.Background(), svc, thread, "Topic A", []string{"tag1"}, "Summarize this", "Context data", "test-model", "Persona")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events, _ := svc.LoadEvents(context.Background(), thread)
	if len(events) == 0 {
		t.Fatal("expected ledger event to be written")
	}

	lastEvent := events[len(events)-1]
	if lastEvent.Type != chat.EventDigest {
		t.Errorf("expected EventDigest, got %s", lastEvent.Type)
	}
	if lastEvent.Content != "Manual summary content" {
		t.Errorf("expected content match, got %s", lastEvent.Content)
	}
	if len(lastEvent.Tags) != 1 || lastEvent.Tags[0] != "tag1" {
		t.Errorf("expected tag1, got %v", lastEvent.Tags)
	}
}

func TestDigestManager_CreateBackgroundDigest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	workspaceRoot := t.TempDir()
	bus := chat.NewEventBus()
	bus.Subscribe(chat.NewLedgerSubscriber())

	svc := workspace.NewService(logger, &coordMockChatEngine{}, workspaceRoot, bus)
	thread, _ := svc.StartThread(context.Background(), "test-bg-digest-thread")

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

	targetWindow := []chat.Event{chat.NewEvent(chat.EventPrompt, "Build auth")}
	count, err := manager.CreateBackgroundDigest(context.Background(), svc, thread, targetWindow, nil, false, "test-model", "Persona")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 digest created, got %d", count)
	}

	events, _ := svc.LoadEvents(context.Background(), thread)
	lastEvent := events[len(events)-1]

	if lastEvent.Type != chat.EventDigest {
		t.Errorf("expected EventDigest, got %s", lastEvent.Type)
	}
	if lastEvent.Content != "Added login" {
		t.Errorf("expected content match, got %s", lastEvent.Content)
	}
	if len(lastEvent.Tags) != 1 || lastEvent.Tags[0] != "security" {
		t.Errorf("expected security tag, got %v", lastEvent.Tags)
	}
}
