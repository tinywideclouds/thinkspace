package workspace_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
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
	factory := func(repoRoot string) workspace.ChatVersioner {
		capturedRoots = append(capturedRoots, repoRoot)
		return &mockFactoryEngine{repoRoot: repoRoot}
	}

	manager := workspace.NewServiceManager(logger, baseRoot, factory, nil, nil)

	golangService := manager.GetService("golang")
	if golangService == nil {
		t.Fatal("expected a valid service for 'golang'")
	}

	expectedRoot := filepath.Join(baseRoot, "golang", "workspace")
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

	manager := workspace.NewServiceManager(logger, baseRoot, func(r string) workspace.ChatVersioner { return nil }, nil, nil)
	ctx := context.Background()

	spaceID := "golang"

	explicitMeta, err := manager.CreateChat(ctx, spaceID, "Architecture Discussion")
	if err != nil {
		t.Fatalf("CreateChat failed: %v", err)
	}
	if explicitMeta.Name != "Architecture Discussion" {
		t.Errorf("Expected explicit name, got %s", explicitMeta.Name)
	}
	if explicitMeta.ActiveBranch != "main" {
		t.Errorf("Expected ActiveBranch 'main', got %s", explicitMeta.ActiveBranch)
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

func TestServiceManager_SpaceSettingsManagement(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	baseRoot := t.TempDir()

	manager := workspace.NewServiceManager(logger, baseRoot, func(r string) workspace.ChatVersioner { return nil }, nil, nil)
	spaceID := "golang"

	settings, err := manager.GetSpaceSettings(spaceID)
	if err != nil {
		t.Fatalf("Expected safe defaults for missing settings, got error: %v", err)
	}
	if !settings.ReviewPolicy.Active {
		t.Errorf("Expected default ReviewPolicy.Active to be true")
	}
	if settings.ReviewPolicy.MinTokens != 200 {
		t.Errorf("Expected default ReviewPolicy.MinTokens to be 200, got %d", settings.ReviewPolicy.MinTokens)
	}
	if settings.ReviewPolicy.IncludePersona {
		t.Errorf("Expected default ReviewPolicy.IncludePersona to be false")
	}
	if !settings.WorkerReviewPolicy.Active {
		t.Errorf("Expected default WorkerReviewPolicy.Active to be true")
	}
	if settings.WorkerReviewPolicy.MinTokens != 200 {
		t.Errorf("Expected default WorkerReviewPolicy.MinTokens to be 200, got %d", settings.WorkerReviewPolicy.MinTokens)
	}

	err = manager.UpdateSpaceSettings(spaceID, func(s *workspace.SpaceSettings) {
		s.ReviewPolicy.Active = false
		s.ReviewPolicy.MinTokens = 500
		s.ReviewPolicy.IncludePersona = true
		s.WorkerReviewPolicy.Active = true
		s.WorkerReviewPolicy.MinTokens = 50
		s.WorkerReviewPolicy.IncludePersona = true
	})
	if err != nil {
		t.Fatalf("UpdateSpaceSettings failed: %v", err)
	}

	updated, err := manager.GetSpaceSettings(spaceID)
	if err != nil {
		t.Fatalf("GetSpaceSettings failed after update: %v", err)
	}
	if updated.ReviewPolicy.Active {
		t.Errorf("Expected ReviewPolicy.Active to be false after update")
	}
	if updated.ReviewPolicy.MinTokens != 500 {
		t.Errorf("Expected ReviewPolicy.MinTokens to be 500, got %d", updated.ReviewPolicy.MinTokens)
	}
	if !updated.ReviewPolicy.IncludePersona {
		t.Errorf("Expected ReviewPolicy.IncludePersona to be true after update")
	}
	if !updated.WorkerReviewPolicy.Active || updated.WorkerReviewPolicy.MinTokens != 50 || !updated.WorkerReviewPolicy.IncludePersona {
		t.Errorf("Expected independent WorkerReviewPolicy update, got %+v", updated.WorkerReviewPolicy)
	}
}

func TestServiceManager_SpaceStateAndErrorPropagation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	baseRoot := t.TempDir()

	manager := workspace.NewServiceManager(logger, baseRoot, func(r string) workspace.ChatVersioner { return nil }, nil, nil)
	spaceID := "golang"

	if err := manager.UpdateSpaceState(spaceID, func(s *workspace.SpaceState) {
		s.ThinkState = "golang"
	}); err != nil {
		t.Fatalf("UpdateSpaceState failed: %v", err)
	}

	state, err := manager.GetSpaceState(spaceID)
	if err != nil {
		t.Fatalf("GetSpaceState failed: %v", err)
	}
	if state.ThinkState != "golang" {
		t.Errorf("expected ThinkState 'golang', got %q", state.ThinkState)
	}

	// Corrupt space.json and verify UpdateSpaceState surfaces the error instead of overwriting it
	spaceJSONPath := filepath.Join(baseRoot, spaceID, "space.json")
	if err := os.WriteFile(spaceJSONPath, []byte("{corrupt json"), 0644); err != nil {
		t.Fatalf("failed writing corrupt space.json: %v", err)
	}
	if err := manager.UpdateSpaceState(spaceID, func(s *workspace.SpaceState) {
		s.ThinkState = "concept"
	}); err == nil {
		t.Errorf("expected UpdateSpaceState to fail on corrupt space.json rather than silently overwriting")
	}

	// Corrupt settings.json and verify UpdateSpaceSettings surfaces the error
	settingsJSONPath := filepath.Join(baseRoot, spaceID, "settings.json")
	if err := os.WriteFile(settingsJSONPath, []byte("{corrupt settings"), 0644); err != nil {
		t.Fatalf("failed writing corrupt settings.json: %v", err)
	}
	if err := manager.UpdateSpaceSettings(spaceID, func(s *workspace.SpaceSettings) {
		s.ReviewPolicy.Active = false
	}); err == nil {
		t.Errorf("expected UpdateSpaceSettings to fail on corrupt settings.json rather than silently overwriting")
	}
}

func TestServiceManager_ChatMetaUpdateListSpacesAndConcurrentCreate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	baseRoot := t.TempDir()
	ctx := context.Background()

	manager := workspace.NewServiceManager(logger, baseRoot, func(r string) workspace.ChatVersioner { return nil }, nil, nil)

	const numConcurrent = 5
	var wg sync.WaitGroup
	errs := make(chan error, numConcurrent)
	for i := 0; i < numConcurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := manager.CreateChat(ctx, "golang", ""); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent CreateChat failed: %v", err)
	}

	chats, err := manager.ListChats(ctx, "golang")
	if err != nil {
		t.Fatalf("ListChats failed: %v", err)
	}
	if len(chats) != numConcurrent {
		t.Fatalf("expected %d chats, got %d", numConcurrent, len(chats))
	}
	seenNames := make(map[string]bool)
	for _, c := range chats {
		if seenNames[c.Name] {
			t.Errorf("duplicate auto-generated chat name %q created under concurrency", c.Name)
		}
		seenNames[c.Name] = true
	}

	targetChat := chats[0]
	if err := manager.UpdateChatMeta("golang", targetChat.ID, func(m *workspace.ChatMeta) {
		m.Name = "Renamed Chat"
		m.ActiveThinkState = "concept"
	}); err != nil {
		t.Fatalf("UpdateChatMeta failed: %v", err)
	}

	loaded, err := manager.GetChatMeta("golang", targetChat.ID)
	if err != nil {
		t.Fatalf("GetChatMeta failed: %v", err)
	}
	if loaded.Name != "Renamed Chat" || loaded.ActiveThinkState != "concept" {
		t.Errorf("unexpected updated ChatMeta: %+v", loaded)
	}

	// Hidden directories (like .graphdb) must be excluded by ListSpaces
	if err := os.MkdirAll(filepath.Join(baseRoot, ".graphdb"), 0755); err != nil {
		t.Fatalf("failed creating .graphdb: %v", err)
	}
	spaces, err := manager.ListSpaces(ctx)
	if err != nil {
		t.Fatalf("ListSpaces failed: %v", err)
	}
	if len(spaces) != 1 || spaces[0] != "golang" {
		t.Errorf("expected spaces [golang], got %v", spaces)
	}
}
