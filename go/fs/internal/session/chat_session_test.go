package session_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/dgraph-io/badger/v4"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/db"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type csMockChatEngine struct{}

func (e *csMockChatEngine) InitChat(ctx context.Context, chatID string) error { return nil }
func (e *csMockChatEngine) Snapshot(ctx context.Context, chatID, message string) (string, error) {
	return "", nil
}
func (e *csMockChatEngine) SpawnCandidateSandbox(ctx context.Context, chatID, candidateID string) (workspace.CandidateSandbox, error) {
	return nil, nil
}
func (e *csMockChatEngine) PreviewCandidate(ctx context.Context, chatID, candidateID string) error {
	return nil
}
func (e *csMockChatEngine) Accept(ctx context.Context, chatID, candidateID, reason string) error {
	return nil
}
func (e *csMockChatEngine) Reject(ctx context.Context, chatID, candidateID, reason string) error {
	return nil
}
func (e *csMockChatEngine) ReadCandidateDiff(ctx context.Context, chatID, candidateID string) (string, error) {
	return "", nil
}

func TestChatSession_InitializationAndMetaFallback(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()
	spaceID := "space-init-test"
	chatID := "chat-123"

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("failed to init badger: %v", err)
	}
	t.Cleanup(func() { graphDB.Close() })

	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatVersioner { return &csMockChatEngine{} }, graphDB, nil)

	_ = os.MkdirAll(filepath.Join(workspaceRoot, spaceID), 0755)

	chatDir := filepath.Join(workspaceRoot, spaceID, "chats", chatID)
	_ = os.MkdirAll(chatDir, 0755)
	_ = os.WriteFile(filepath.Join(chatDir, "meta.json"), []byte(`{"id":"`+chatID+`", "name":"`+chatID+`", "active_branch": "main"}`), 0644)

	_ = svcManager.UpdateSpaceState(spaceID, func(s *workspace.SpaceState) { s.ThinkState = "golang" })

	registry := config.NewRegistry()

	chatSession, err := session.NewChatSession(
		ctx,
		logger,
		registry,
		svcManager,
		nil,
		nil,
		nil,
		spaceID,
		chatID,
	)

	if err != nil {
		t.Fatalf("Failed to initialize ChatSession: %v", err)
	}

	if chatSession.ChatID != chatID {
		t.Errorf("Expected ChatID %s, got %s", chatID, chatSession.ChatID)
	}
	if chatSession.SpaceID != spaceID {
		t.Errorf("Expected SpaceID %s, got %s", spaceID, chatSession.SpaceID)
	}

	if chatSession.Meta.ActiveThinkState != "golang" {
		t.Errorf("Expected initialized ActiveThinkState to be 'golang', got %s", chatSession.Meta.ActiveThinkState)
	}

	mu := chatSession.Mu()
	if mu == nil {
		t.Errorf("Expected Mu() to return a valid RWMutex pointer")
	}
}

func TestChatSession_SetReviewPolicy_NilSafety(t *testing.T) {
	chatSession := &session.ChatSession{}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("SetReviewPolicy panicked when given a nil policy: %v", r)
		}
	}()

	chatSession.SetReviewPolicy(nil)

	validPolicy := session.NewTokenThresholdPolicy()
	chatSession.SetReviewPolicy(validPolicy)
}

func TestChatSession_GetGraphData_MissingThinkState(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()
	spaceID := "space-missing-state"
	chatID := "chat-456"

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, _ := db.NewBadgerGraph(opts)
	t.Cleanup(func() { graphDB.Close() })

	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatVersioner { return &csMockChatEngine{} }, graphDB, nil)
	_ = os.MkdirAll(filepath.Join(workspaceRoot, spaceID), 0755)

	chatDir := filepath.Join(workspaceRoot, spaceID, "chats", chatID)
	_ = os.MkdirAll(chatDir, 0755)
	_ = os.WriteFile(filepath.Join(chatDir, "meta.json"), []byte(`{"id":"`+chatID+`", "name":"`+chatID+`", "active_branch": "main"}`), 0644)

	registry := config.NewRegistry()

	chatSession, err := session.NewChatSession(
		ctx,
		logger,
		registry,
		svcManager,
		nil,
		nil,
		nil,
		spaceID,
		chatID,
	)
	if err != nil {
		t.Fatalf("Failed to initialize ChatSession: %v", err)
	}

	_, err = chatSession.GetGraphData(ctx)
	if err != nil {
		t.Fatalf("GetGraphData failed unexpectedly when think state was missing: %v", err)
	}
}
