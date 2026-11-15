package workspace_test

import (
	"bufio"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type mockSandbox struct {
	writeCalled    bool
	readCalled     bool
	execCalled     bool
	applyCalled    bool
	deliverCalled  bool
	tearDownCalled bool
	files          map[string][]byte
}

func newMockSandbox() *mockSandbox {
	return &mockSandbox{
		files: make(map[string][]byte),
	}
}

func (m *mockSandbox) ResolvePath(path string) string {
	return path
}

func (m *mockSandbox) WriteFile(ctx context.Context, path string, data []byte) error {
	m.writeCalled = true
	m.files[path] = data
	return nil
}

func (m *mockSandbox) ReadFile(ctx context.Context, path string) ([]byte, error) {
	m.readCalled = true
	return m.files[path], nil
}

func (m *mockSandbox) ExecuteCommand(ctx context.Context, command string, args ...string) (string, error) {
	m.execCalled = true
	return "mock output", nil
}

func (m *mockSandbox) ApplyDraft(ctx context.Context, message string) error {
	m.applyCalled = true
	return nil
}

func (m *mockSandbox) DeliverForReview(ctx context.Context) error {
	m.deliverCalled = true
	return nil
}

func (m *mockSandbox) TearDown(ctx context.Context) error {
	m.tearDownCalled = true
	return nil
}

type mockChatEngine struct {
	initCalled    bool
	spawnCalled   bool
	previewCalled bool
	acceptCalled  bool
	rejectCalled  bool
	lastSandbox   *mockSandbox
}

func (m *mockChatEngine) InitChat(ctx context.Context, chatID string) error {
	m.initCalled = true
	return nil
}

func (m *mockChatEngine) Snapshot(ctx context.Context, chatID string, message string) (string, error) {
	return "mock-sha", nil
}

func (m *mockChatEngine) SpawnCandidateSandbox(ctx context.Context, chatID string, candidateID string) (workspace.CandidateSandbox, error) {
	m.spawnCalled = true
	m.lastSandbox = newMockSandbox()
	return m.lastSandbox, nil
}

func (m *mockChatEngine) PreviewCandidate(ctx context.Context, chatID string, candidateID string) error {
	m.previewCalled = true
	return nil
}

func (m *mockChatEngine) Accept(ctx context.Context, chatID string, candidateID string, reason string) error {
	m.acceptCalled = true
	return nil
}

func (m *mockChatEngine) Reject(ctx context.Context, chatID string, candidateID string, reason string) error {
	m.rejectCalled = true
	return nil
}

func (m *mockChatEngine) ReadCandidateDiff(ctx context.Context, chatID, candidateID string) (string, error) {
	return "+ mock diff", nil
}

func setupServiceTest(t *testing.T) (*workspace.Service, *mockChatEngine, string) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	mockEngine := &mockChatEngine{}
	workspaceRoot := t.TempDir()

	ledger := thinkgraph.NewFileLedger(10, workspaceRoot)
	dispatcher := thinkgraph.NewDispatcher(nil, ledger)

	svc := workspace.NewService(logger, mockEngine, workspaceRoot, dispatcher, nil)
	return svc, mockEngine, workspaceRoot
}

func TestService_StartChat(t *testing.T) {
	svc, mockEngine, _ := setupServiceTest(t)
	ctx := context.Background()

	chat := &thinkgraph.ChatAddress{
		ChatID:    "test-chat",
		ProjectID: "test-space",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}
	err := svc.InitializeChat(ctx, chat)
	if err != nil {
		t.Fatalf("InitializeChat failed: %v", err)
	}

	if !mockEngine.initCalled {
		t.Errorf("Expected ChatEngine.InitChat to be called")
	}
}

func TestService_ResolveCandidate(t *testing.T) {
	svc, mockEngine, _ := setupServiceTest(t)
	ctx := context.Background()

	chat := &thinkgraph.ChatAddress{
		ChatID:    "test-chat",
		ProjectID: "test-space",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}
	_ = svc.InitializeChat(ctx, chat)

	err := svc.ResolveCandidate(ctx, chat, "cand-123", true, "Looks good")
	if err != nil {
		t.Fatalf("ResolveCandidate failed: %v", err)
	}

	if !mockEngine.acceptCalled {
		t.Errorf("Expected ChatEngine.Accept to be called")
	}

	err = svc.ResolveCandidate(ctx, chat, "cand-123", false, "Needs work")
	if err != nil {
		t.Fatalf("ResolveCandidate failed: %v", err)
	}

	if !mockEngine.rejectCalled {
		t.Errorf("Expected ChatEngine.Reject to be called")
	}
}

func TestService_LedgerTags(t *testing.T) {
	svc, _, workspaceRoot := setupServiceTest(t)
	ctx := context.Background()

	chat := &thinkgraph.ChatAddress{
		ChatID:    "tag-chat",
		ProjectID: "test-space",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}
	_ = svc.InitializeChat(ctx, chat)

	tags := []string{"ui", "frontend"}

	ev := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Test tags")
	ev.Tags = tags
	if err := svc.Dispatcher().AppendEvent(ctx, chat, ev); err != nil {
		t.Fatalf("AppendEvent failed: %v", err)
	}

	logsDir := filepath.Join(workspaceRoot, chat.ProjectID, "chats", chat.ChatID, "ledger", "logs", chat.Name)
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		t.Fatalf("failed to read logs dir: %v", err)
	}

	var shards []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			shards = append(shards, e.Name())
		}
	}
	sort.Strings(shards)

	var recoveredEvents []thinkgraph.Event
	for _, shard := range shards {
		file, _ := os.Open(filepath.Join(logsDir, shard))
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			if len(scanner.Bytes()) > 0 {
				var event thinkgraph.Event
				json.Unmarshal(scanner.Bytes(), &event)
				recoveredEvents = append(recoveredEvents, event)
			}
		}
		if err := scanner.Err(); err != nil {
			file.Close()
			t.Fatalf("failed to scan ledger shard %s: %v", shard, err)
		}
		file.Close()
	}

	if len(recoveredEvents) != 1 {
		t.Fatalf("expected exactly 1 event in ledger, got %d", len(recoveredEvents))
	}

	if len(recoveredEvents[0].Tags) != 2 || recoveredEvents[0].Tags[0] != "ui" {
		t.Errorf("expected tags [ui, frontend] to be recovered from ledger, got %v", recoveredEvents[0].Tags)
	}
}
