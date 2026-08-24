package workspace_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type mockStateEngine struct {
	initCalled    bool
	spawnCalled   bool
	commitCalled  bool
	submitCalled  bool
	previewCalled bool
	acceptCalled  bool
	rejectCalled  bool
	closeCalled   bool
}

func (m *mockStateEngine) InitThread(ctx context.Context, mainDir string, threadID string) error {
	m.initCalled = true
	return nil
}

func (m *mockStateEngine) Snapshot(ctx context.Context, mainDir string, threadID string, message string) (string, error) {
	return "mock-sha", nil
}

func (m *mockStateEngine) SpawnSandbox(ctx context.Context, mainDir string, threadID string, candidateID string) (string, error) {
	m.spawnCalled = true
	return filepath.Join(mainDir, "mock-sandbox"), nil
}

func (m *mockStateEngine) CommitSandbox(ctx context.Context, sandboxDir string, message string) (string, error) {
	m.commitCalled = true
	return "mock-commit-sha", nil
}

func (m *mockStateEngine) SubmitSandbox(ctx context.Context, sandboxDir string, mainDir string, candidateID string) error {
	m.submitCalled = true
	return nil
}

func (m *mockStateEngine) CloseSandbox(ctx context.Context, sandboxDir string) error {
	m.closeCalled = true
	return nil
}

func (m *mockStateEngine) PreviewCandidate(ctx context.Context, mainDir string, threadID string, candidateID string) error {
	m.previewCalled = true
	return nil
}

func (m *mockStateEngine) Accept(ctx context.Context, mainDir string, threadID string, candidateID string, reason string) error {
	m.acceptCalled = true
	return nil
}

func (m *mockStateEngine) Reject(ctx context.Context, mainDir string, threadID string, candidateID string, reason string) error {
	m.rejectCalled = true
	return nil
}

func (m *mockStateEngine) ReadCandidateDiff(ctx context.Context, repoRoot, threadID, candidateID string) (string, error) {
	return "+ mock diff", nil
}

func setupServiceTest(t *testing.T) (*workspace.Service, *mockStateEngine, string) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	mockEngine := &mockStateEngine{}
	workspaceRoot := t.TempDir()
	svc := workspace.NewService(logger, mockEngine, workspaceRoot)
	return svc, mockEngine, workspaceRoot
}

func TestService_StartThread(t *testing.T) {
	svc, mockEngine, _ := setupServiceTest(t)
	ctx := context.Background()

	thread, err := svc.StartThread(ctx, "test-thread")
	if err != nil {
		t.Fatalf("StartThread failed: %v", err)
	}

	if !mockEngine.initCalled {
		t.Errorf("Expected StateEngine.InitThread to be called")
	}

	if thread.ID != "test-thread" {
		t.Errorf("Expected thread ID 'test-thread', got '%s'", thread.ID)
	}

	if _, err := os.Stat(thread.LedgerPath); os.IsNotExist(err) {
		t.Errorf("Expected ledger file to be created at %s", thread.LedgerPath)
	}
}

func TestService_ProposeAndResolveCandidate(t *testing.T) {
	svc, mockEngine, _ := setupServiceTest(t)
	ctx := context.Background()

	thread, _ := svc.StartThread(ctx, "test-thread")

	files := map[string][]byte{
		"main.go": []byte("package main"),
	}

	candidate, err := svc.ProposeCandidate(ctx, thread, "test_tool", files)
	if err != nil {
		t.Fatalf("ProposeCandidate failed: %v", err)
	}

	if !mockEngine.spawnCalled || !mockEngine.commitCalled || !mockEngine.submitCalled || !mockEngine.previewCalled || !mockEngine.closeCalled {
		t.Errorf("Expected StateEngine lifecycle methods to be called during proposal")
	}

	if candidate.Status != workspace.StatusPending {
		t.Errorf("Expected candidate status to be pending")
	}

	err = svc.ResolveCandidate(ctx, thread, candidate.ID, true, "Looks good")
	if err != nil {
		t.Fatalf("ResolveCandidate failed: %v", err)
	}

	if !mockEngine.acceptCalled {
		t.Errorf("Expected StateEngine.Accept to be called")
	}

	err = svc.ResolveCandidate(ctx, thread, candidate.ID, false, "Needs work")
	if err != nil {
		t.Fatalf("ResolveCandidate failed: %v", err)
	}

	if !mockEngine.rejectCalled {
		t.Errorf("Expected StateEngine.Reject to be called")
	}
}
