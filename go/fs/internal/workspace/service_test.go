package workspace_test

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// ... (Keep existing mocks: mockSandbox, mockChatEngine, setupServiceTest) ...
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

	bus := chat.NewEventBus()
	// Must attach subscriber to physically write to disk for LoadEvents to work
	bus.Subscribe(chat.NewLedgerSubscriber())

	svc := workspace.NewService(logger, mockEngine, workspaceRoot, bus)
	return svc, mockEngine, workspaceRoot
}

// ... (Keep existing TestService_StartThread, TestService_ProposeAndResolveCandidate, TestService_Receipts) ...
func TestService_StartThread(t *testing.T) {
	svc, mockEngine, _ := setupServiceTest(t)
	ctx := context.Background()

	thread, err := svc.StartThread(ctx, "test-thread")
	if err != nil {
		t.Fatalf("StartThread failed: %v", err)
	}

	if !mockEngine.initCalled {
		t.Errorf("Expected ChatEngine.InitChat to be called")
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

	if !mockEngine.spawnCalled || !mockEngine.previewCalled {
		t.Errorf("Expected ChatEngine lifecycle methods to be called during proposal")
	}

	if mockEngine.lastSandbox == nil || !mockEngine.lastSandbox.writeCalled || !mockEngine.lastSandbox.applyCalled || !mockEngine.lastSandbox.deliverCalled || !mockEngine.lastSandbox.tearDownCalled {
		t.Errorf("Expected CandidateSandbox virtual file I/O and lifecycle methods to be called")
	}

	if candidate.Status != workspace.StatusPending {
		t.Errorf("Expected candidate status to be pending")
	}

	err = svc.ResolveCandidate(ctx, thread, candidate.ID, true, "Looks good")
	if err != nil {
		t.Fatalf("ResolveCandidate failed: %v", err)
	}

	if !mockEngine.acceptCalled {
		t.Errorf("Expected ChatEngine.Accept to be called")
	}

	err = svc.ResolveCandidate(ctx, thread, candidate.ID, false, "Needs work")
	if err != nil {
		t.Fatalf("ResolveCandidate failed: %v", err)
	}

	if !mockEngine.rejectCalled {
		t.Errorf("Expected ChatEngine.Reject to be called")
	}
}

func TestService_Receipts(t *testing.T) {
	svc, _, _ := setupServiceTest(t)
	ctx := context.Background()

	thread, _ := svc.StartThread(ctx, "receipt-thread")

	receipt := workspace.FlowReceipt{
		FlowID:    "flow-999",
		TaskID:    thread.ID,
		Timestamp: time.Now().UTC(),
		Task:      "Test Serialization",
		Summary:   "Saved successfully",
		Agents: []workspace.AgentRecord{
			{
				AgentID:     "agent-1",
				Passed:      true,
				Instruction: "Do work",
			},
		},
	}

	if err := svc.SaveReceipt(ctx, thread, receipt); err != nil {
		t.Fatalf("SaveReceipt failed: %v", err)
	}

	data, err := svc.GetReceipt(ctx, thread, "flow-999")
	if err != nil {
		t.Fatalf("GetReceipt failed: %v", err)
	}

	xmlStr := string(data)
	if !strings.Contains(xmlStr, `FlowID="flow-999"`) {
		t.Errorf("Expected FlowID in XML, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `<Task>Test Serialization</Task>`) {
		t.Errorf("Expected Task description in XML, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `AgentID="agent-1"`) {
		t.Errorf("Expected AgentRecord in XML, got: %s", xmlStr)
	}
}

// Phase 3 Coverage: Verify Tags map safely to the event and persist to disk
func TestService_LedgerTags(t *testing.T) {
	svc, _, _ := setupServiceTest(t)
	ctx := context.Background()

	thread, _ := svc.StartThread(ctx, "tag-thread")

	tags := []string{"ui", "frontend"}
	if err := svc.LogProposal(ctx, thread, "cand-123", "Added UI", tags); err != nil {
		t.Fatalf("LogProposal failed: %v", err)
	}

	events, err := svc.LoadEvents(ctx, thread)
	if err != nil {
		t.Fatalf("LoadEvents failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected exactly 1 event in ledger, got %d", len(events))
	}

	if len(events[0].Tags) != 2 || events[0].Tags[0] != "ui" {
		t.Errorf("expected tags [ui, frontend] to be recovered from ledger, got %v", events[0].Tags)
	}
}

func TestService_FetchEvents(t *testing.T) {
	svc, _, _ := setupServiceTest(t)
	ctx := context.Background()

	thread, _ := svc.StartThread(ctx, "fetch-thread")

	_ = svc.LogUserMessage(ctx, thread, "Event A")
	_ = svc.LogModelResponse(ctx, thread, "Event B")
	_ = svc.LogUserMessage(ctx, thread, "Event C")

	allEvents, _ := svc.LoadEvents(ctx, thread)
	if len(allEvents) != 3 {
		t.Fatalf("Expected 3 events in ledger")
	}

	targetID1 := allEvents[0].ID.String()
	targetID2 := allEvents[2].ID.String()

	fetched, err := svc.FetchEvents(ctx, thread, []string{targetID1, targetID2})
	if err != nil {
		t.Fatalf("FetchEvents failed: %v", err)
	}

	if len(fetched) != 2 {
		t.Fatalf("Expected exactly 2 events fetched, got %d", len(fetched))
	}

	if fetched[0].ID.String() != targetID1 && fetched[1].ID.String() != targetID1 {
		t.Errorf("Expected to retrieve targetID1")
	}
	if fetched[0].ID.String() != targetID2 && fetched[1].ID.String() != targetID2 {
		t.Errorf("Expected to retrieve targetID2")
	}
}
