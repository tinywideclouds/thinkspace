package workspace_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type mockFactoryEngine struct {
	repoRoot string
}

func (m *mockFactoryEngine) InitChat(ctx context.Context, chatID string) error { return nil }
func (m *mockFactoryEngine) Snapshot(ctx context.Context, chatID string, message string) (string, error) {
	return "", nil
}
func (m *mockFactoryEngine) SpawnCandidateSandbox(ctx context.Context, chatID string, candidateID string) (workspace.CandidateSandbox, error) {
	return nil, nil
}
func (m *mockFactoryEngine) PreviewCandidate(ctx context.Context, chatID string, candidateID string) error {
	return nil
}
func (m *mockFactoryEngine) ReadCandidateDiff(ctx context.Context, chatID string, candidateID string) (string, error) {
	return "", nil
}
func (m *mockFactoryEngine) Accept(ctx context.Context, chatID string, candidateID string, reason string) error {
	return nil
}
func (m *mockFactoryEngine) Reject(ctx context.Context, chatID string, candidateID string, reason string) error {
	return nil
}

func TestServiceManager_LazyLoading(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	baseRoot := t.TempDir()

	var capturedRoots []string
	factory := func(repoRoot string) workspace.ChatEngine {
		capturedRoots = append(capturedRoots, repoRoot)
		return &mockFactoryEngine{repoRoot: repoRoot}
	}

	manager := workspace.NewServiceManager(logger, baseRoot, factory)

	golangService := manager.GetService("golang")
	if golangService == nil {
		t.Fatal("expected a valid service for 'golang'")
	}

	expectedRoot := filepath.Join(baseRoot, "golang")
	if len(capturedRoots) != 1 || capturedRoots[0] != expectedRoot {
		t.Errorf("factory did not receive correct repo root. Expected %s, got %v", expectedRoot, capturedRoots)
	}

	cachedService := manager.GetService("golang")
	if cachedService != golangService {
		t.Error("manager did not return the cached service pointer")
	}

	pythonService := manager.GetService("python")
	if pythonService == golangService {
		t.Error("manager returned the same service pointer for different spaces")
	}
}

func TestServiceManager_ChatManagement(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	baseRoot := t.TempDir()
	manager := workspace.NewServiceManager(logger, baseRoot, func(r string) workspace.ChatEngine { return nil })
	ctx := context.Background()

	spaceID := "golang"

	explicitMeta, err := manager.CreateChat(ctx, spaceID, "Architecture Discussion")
	if err != nil {
		t.Fatalf("CreateChat failed: %v", err)
	}
	if explicitMeta.Name != "Architecture Discussion" {
		t.Errorf("Expected explicit name, got %s", explicitMeta.Name)
	}

	autoMeta1, err := manager.CreateChat(ctx, spaceID, "")
	if err != nil {
		t.Fatalf("CreateChat failed: %v", err)
	}
	if autoMeta1.Name != "New Conversation" {
		t.Errorf("Expected 'New Conversation', got '%s'", autoMeta1.Name)
	}

	autoMeta2, err := manager.CreateChat(ctx, spaceID, " ")
	if err != nil {
		t.Fatalf("CreateChat failed: %v", err)
	}
	if autoMeta2.Name != "New Conversation (1)" {
		t.Errorf("Expected 'New Conversation (1)', got '%s'", autoMeta2.Name)
	}

	chats, err := manager.ListChats(ctx, spaceID)
	if err != nil {
		t.Fatalf("ListChats failed: %v", err)
	}
	if len(chats) != 3 {
		t.Fatalf("Expected 3 chats, got %d", len(chats))
	}

	if chats[0].ID != autoMeta2.ID {
		t.Errorf("Expected newest chat first in list")
	}
	if chats[2].ID != explicitMeta.ID {
		t.Errorf("Expected oldest chat last in list")
	}
}
