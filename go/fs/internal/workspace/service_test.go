package workspace_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/dgraph-io/badger/v4"
	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/db"
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
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	mockEngine := &mockChatEngine{}
	workspaceRoot := t.TempDir()

	ledger := thinkgraph.NewFileLedger(10, workspaceRoot)
	dispatcher := thinkgraph.NewDispatcher(nil, ledger)

	svc := workspace.NewService(logger, mockEngine, workspaceRoot, dispatcher, nil)
	return svc, mockEngine, workspaceRoot
}

func setupServiceTestWithGraph(t *testing.T) (*workspace.Service, *mockChatEngine, *db.BadgerGraph, string) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	mockEngine := &mockChatEngine{}
	workspaceRoot := t.TempDir()

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("failed to init badger: %v", err)
	}
	t.Cleanup(func() { _ = graphDB.Close() })

	graphStore := thinkgraph.NewGraphStore(graphDB, nil)
	ledger := thinkgraph.NewFileLedger(10, workspaceRoot)
	dispatcher := thinkgraph.NewDispatcher(graphStore, ledger)

	svc := workspace.NewService(logger, mockEngine, workspaceRoot, dispatcher, nil)
	return svc, mockEngine, graphDB, workspaceRoot
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
	if err := svc.InitializeChat(ctx, chat); err != nil {
		t.Fatalf("InitializeChat failed: %v", err)
	}

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
	if err := svc.InitializeChat(ctx, chat); err != nil {
		t.Fatalf("InitializeChat failed: %v", err)
	}

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
		file, openErr := os.Open(filepath.Join(logsDir, shard))
		if openErr != nil {
			t.Fatalf("failed opening shard %s: %v", shard, openErr)
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			if len(scanner.Bytes()) > 0 {
				var event thinkgraph.Event
				if unmarshalErr := json.Unmarshal(scanner.Bytes(), &event); unmarshalErr != nil {
					file.Close()
					t.Fatalf("failed unmarshaling event: %v", unmarshalErr)
				}
				recoveredEvents = append(recoveredEvents, event)
			}
		}
		if scanErr := scanner.Err(); scanErr != nil {
			file.Close()
			t.Fatalf("scanner error: %v", scanErr)
		}
		if closeErr := file.Close(); closeErr != nil {
			t.Fatalf("failed closing shard file: %v", closeErr)
		}
	}

	if len(recoveredEvents) != 1 {
		t.Fatalf("expected exactly 1 event in ledger, got %d", len(recoveredEvents))
	}

	if len(recoveredEvents[0].Tags) != 2 || recoveredEvents[0].Tags[0] != "ui" {
		t.Errorf("expected tags [ui, frontend] to be recovered from ledger, got %v", recoveredEvents[0].Tags)
	}
}

func TestService_AppendDigest(t *testing.T) {
	svc, _, graphDB, _ := setupServiceTestWithGraph(t)
	ctx := context.Background()

	chat := &thinkgraph.ChatAddress{
		ChatID:    "digest-chat",
		ProjectID: "test-space",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}
	if err := svc.InitializeChat(ctx, chat); err != nil {
		t.Fatalf("InitializeChat failed: %v", err)
	}

	summarized := []uuid.UUID{uuid.NewV7()}
	if err := svc.AppendDigest(ctx, chat, "HTTP server architecture", []string{"golang", "http"}, true, summarized); err != nil {
		t.Fatalf("AppendDigest failed: %v", err)
	}

	digests, err := graphDB.GetDigests(ctx, chat)
	if err != nil {
		t.Fatalf("GetDigests failed: %v", err)
	}
	if len(digests) != 1 {
		t.Fatalf("expected 1 digest, got %d", len(digests))
	}
	if digests[0].Summary != "HTTP server architecture" || !digests[0].Promoted {
		t.Errorf("unexpected digest metadata: %+v", digests[0])
	}
}

func TestService_SyncGraphContext_FastForwardAndBranchedAncestry(t *testing.T) {
	svc, _, graphDB, _ := setupServiceTestWithGraph(t)
	ctx := context.Background()

	mainAddr := &thinkgraph.ChatAddress{
		ChatID:    "sync-chat",
		ProjectID: "test-space",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}
	if err := svc.InitializeChat(ctx, mainAddr); err != nil {
		t.Fatalf("InitializeChat failed: %v", err)
	}

	if err := svc.AppendUserPrompt(ctx, mainAddr, "Initial prompt on main"); err != nil {
		t.Fatalf("AppendUserPrompt failed: %v", err)
	}

	// Create a child branch anchored to main
	branchAddr := &thinkgraph.ChatAddress{
		ChatID:    "sync-chat",
		ProjectID: "test-space",
		SubGraph: thinkgraph.SubGraph{
			Name:    "feature-branch",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}
	if err := svc.AppendBranchAnchor(ctx, branchAddr, "main", uuid.NewV7()); err != nil {
		t.Fatalf("AppendBranchAnchor failed: %v", err)
	}
	if err := svc.AppendUserPrompt(ctx, branchAddr, "Prompt on feature-branch"); err != nil {
		t.Fatalf("AppendUserPrompt(branch) failed: %v", err)
	}

	// Wipe BadgerDB and verify SyncGraphContext on the child branch rehydrates both main and feature-branch
	if err := graphDB.ClearChatGraph(ctx, branchAddr); err != nil {
		t.Fatalf("ClearChatGraph failed: %v", err)
	}

	if err := svc.SyncGraphContext(ctx, branchAddr); err != nil {
		t.Fatalf("SyncGraphContext failed: %v", err)
	}

	mainTimeline, err := graphDB.GetSubGraphTimeline(ctx, mainAddr, mainAddr.GraphID)
	if err != nil {
		t.Fatalf("GetSubGraphTimeline(main) failed: %v", err)
	}
	if len(mainTimeline) != 1 {
		t.Errorf("expected 1 event restored on main via ancestry walk, got %d", len(mainTimeline))
	}

	branchTimeline, err := graphDB.GetSubGraphTimeline(ctx, branchAddr, branchAddr.GraphID)
	if err != nil {
		t.Fatalf("GetSubGraphTimeline(branch) failed: %v", err)
	}
	if len(branchTimeline) != 2 {
		t.Errorf("expected 2 events restored on feature-branch, got %d", len(branchTimeline))
	}
}

func TestService_ContextPreviewSaveGetAndPrune(t *testing.T) {
	svc, _, _ := setupServiceTest(t)
	ctx := context.Background()

	chat := &thinkgraph.ChatAddress{
		ChatID:    "preview-chat",
		ProjectID: "test-space",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}

	var ids []string
	for i := 0; i < 17; i++ {
		id, err := svc.SaveContextPreview(ctx, chat, fmt.Sprintf("preview-%d", i))
		if err != nil {
			t.Fatalf("SaveContextPreview %d failed: %v", i, err)
		}
		ids = append(ids, id)
	}

	// Latest preview must be readable
	latestContent, err := svc.GetContextPreview(ctx, chat, ids[len(ids)-1])
	if err != nil {
		t.Fatalf("GetContextPreview failed: %v", err)
	}
	if string(latestContent) != "preview-16" {
		t.Errorf("expected 'preview-16', got %q", string(latestContent))
	}

	// Oldest pruned preview must no longer exist
	if _, err := svc.GetContextPreview(ctx, chat, ids[0]); err == nil {
		t.Errorf("expected oldest preview %s to be pruned", ids[0])
	}

	// Path traversal must be rejected
	if _, err := svc.GetContextPreview(ctx, chat, "../meta.json"); err == nil {
		t.Errorf("expected path traversal preview ID to be rejected")
	}
}

func TestService_NilDependencyGuards(t *testing.T) {
	svc := workspace.NewService(nil, nil, t.TempDir(), nil, nil)
	ctx := context.Background()
	chat := &thinkgraph.ChatAddress{ChatID: "c", ProjectID: "p"}

	if err := svc.InitializeChat(ctx, chat); err == nil {
		t.Errorf("expected error when engine is nil")
	}
	if err := svc.AppendUserPrompt(ctx, chat, "hi"); err == nil {
		t.Errorf("expected error when dispatcher is nil")
	}
	if err := svc.AppendDigest(ctx, chat, "sum", nil, false, nil); err == nil {
		t.Errorf("expected error when dispatcher is nil")
	}
}
