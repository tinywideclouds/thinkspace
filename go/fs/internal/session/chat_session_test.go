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

// csMockChatEngine provides a safe no-op implementation of ChatVersioner for initialization tests.
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

	// Create the workspace directory structure
	_ = os.MkdirAll(filepath.Join(workspaceRoot, spaceID), 0755)

	// Scaffold base space state so the ChatSession can inherit the default ThinkState on boot
	_ = svcManager.UpdateSpaceState(spaceID, func(s *workspace.SpaceState) { s.ThinkState = "golang" })

	registry := config.NewRegistry()

	chatSession, err := session.NewChatSession(
		ctx,
		logger,
		registry,
		svcManager,
		nil, // modelClient not required just for init
		nil, // fanOutFlow
		nil, // digestManager
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

	// Verify that the ChatSession correctly pulled the fallback ActiveThinkState from the SpaceState
	// because the newly created chat did not have one explicitly defined in its meta.json yet.
	if chatSession.Meta.ActiveThinkState != "golang" {
		t.Errorf("Expected initialized ActiveThinkState to be 'golang', got %s", chatSession.Meta.ActiveThinkState)
	}

	// Verify the mutex is safely addressable
	mu := chatSession.Mu()
	if mu == nil {
		t.Errorf("Expected Mu() to return a valid RWMutex pointer")
	}
}

func TestChatSession_SetReviewPolicy_NilSafety(t *testing.T) {
	// Create a zero-value session just to test the setter's nil protection
	chatSession := &session.ChatSession{}

	// Ensure setting a nil policy safely falls back to the default NewTokenThresholdPolicy
	// without causing a panic when locked.
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

	// The registry is empty, so attempting to fetch graph data should safely warn
	// and fall back to a zero-value bundle assembly without panicking.
	_, err = chatSession.GetGraphData(ctx)
	if err != nil {
		t.Fatalf("GetGraphData failed unexpectedly when think state was missing: %v", err)
	}
}
