package flows_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"google.golang.org/genai"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace/flows"
)

// --- Mocks ---

type mockChatEngine struct {
	sandbox *mockSandbox
}

func (m *mockChatEngine) InitChat(ctx context.Context, chatID string) error { return nil }
func (m *mockChatEngine) Snapshot(ctx context.Context, chatID string, msg string) (string, error) {
	return "", nil
}
func (m *mockChatEngine) PreviewCandidate(ctx context.Context, chatID string, candidateID string) error {
	return nil
}
func (m *mockChatEngine) ReadCandidateDiff(ctx context.Context, chatID string, candidateID string) (string, error) {
	return "", nil
}
func (m *mockChatEngine) Accept(ctx context.Context, chatID string, candidateID string, reason string) error {
	return nil
}
func (m *mockChatEngine) Reject(ctx context.Context, chatID string, candidateID string, reason string) error {
	return nil
}

func (m *mockChatEngine) SpawnCandidateSandbox(ctx context.Context, chatID string, candidateID string) (workspace.CandidateSandbox, error) {
	m.sandbox = &mockSandbox{candidateID: candidateID}
	return m.sandbox, nil
}

type mockSandbox struct {
	candidateID string
	delivered   bool
}

func (m *mockSandbox) WriteFile(ctx context.Context, path string, data []byte) error { return nil }
func (m *mockSandbox) ReadFile(ctx context.Context, path string) ([]byte, error)     { return nil, nil }
func (m *mockSandbox) ExecuteCommand(ctx context.Context, command string, args ...string) (string, error) {
	return "", nil
}
func (m *mockSandbox) ApplyDraft(ctx context.Context, message string) error { return nil }
func (m *mockSandbox) DeliverForReview(ctx context.Context) error {
	m.delivered = true
	return nil
}
func (m *mockSandbox) TearDown(ctx context.Context) error { return nil }

type mockThinkSpace struct{}

func (m *mockThinkSpace) Name() string                                  { return "mock" }
func (m *mockThinkSpace) SystemPrompt() string                          { return "" }
func (m *mockThinkSpace) SubAgentSystemPrompt() string                  { return "" }
func (m *mockThinkSpace) Model(category workspace.ModelCategory) string { return "" }
func (m *mockThinkSpace) TurnTimeout() time.Duration                    { return 0 }
func (m *mockThinkSpace) AgentTimeout() time.Duration                   { return 0 }
func (m *mockThinkSpace) VerifyTimeout() time.Duration                  { return 0 }

func (m *mockThinkSpace) Tools() []*genai.Tool         { return nil }
func (m *mockThinkSpace) Verifier() workspace.Verifier { return nil }

type mockVerifier struct {
	errsToReturn []error
	calls        int
}

func (m *mockVerifier) Verify(ctx context.Context, sandbox workspace.CandidateSandbox) error {
	if m.calls < len(m.errsToReturn) {
		err := m.errsToReturn[m.calls]
		m.calls++
		return err
	}
	m.calls++
	return nil
}

type mockEmitter struct {
	events []flows.FlowEvent
}

func (m *mockEmitter) Emit(event flows.FlowEvent) {
	m.events = append(m.events, event)
}

// --- Tests ---

func TestFanOutFlow_CleanRun(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	flow := flows.NewFanOutFlow(logger)

	engine := &mockChatEngine{}
	workspaceRoot := t.TempDir()
	service := workspace.NewService(logger, engine, workspaceRoot)
	thread, _ := service.StartThread(context.Background(), "chat-1")

	verifier := &mockVerifier{}
	emitter := &mockEmitter{}
	space := &mockThinkSpace{}

	args := map[string]any{
		"agent_instructions": []any{"Do a thing"},
	}
	flowCfg := flows.FlowConfig{RetryPrompt: "Trace: {{.ErrorTrace}}"}
	flowCtx := flows.FlowContext{FlowID: "flow-123", SpaceID: "golang"}

	var promptsReceived []string
	executor := func(ctx context.Context, instructions string, sandbox workspace.CandidateSandbox, agentID int, tokenChan chan<- workspace.AgentToken) error {
		promptsReceived = append(promptsReceived, instructions)
		return nil
	}

	result, err := flow.Execute(context.Background(), service, thread, space, args, flowCfg, flowCtx, emitter, executor, verifier)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Branches) != 1 || result.Branches[0] != "candidate/chat-1-agent-1" {
		t.Errorf("expected candidate/chat-1-agent-1, got %v", result.Branches)
	}

	if !engine.sandbox.delivered {
		t.Errorf("expected sandbox to be delivered")
	}

	if len(emitter.events) < 4 {
		t.Fatalf("expected at least 4 events, got %d", len(emitter.events))
	}
	if emitter.events[0].Type != flows.FlowStart {
		t.Errorf("expected first event to be FlowStart")
	}
	lastEvent := emitter.events[len(emitter.events)-1]
	if lastEvent.Type != flows.FlowComplete || !lastEvent.Passed {
		t.Errorf("expected last event to be successful FlowComplete")
	}
}

func TestFanOutFlow_RetryInjectsContext(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	flow := flows.NewFanOutFlow(logger)

	engine := &mockChatEngine{}
	workspaceRoot := t.TempDir()
	service := workspace.NewService(logger, engine, workspaceRoot)
	thread, _ := service.StartThread(context.Background(), "chat-1")

	verifier := &mockVerifier{errsToReturn: []error{errors.New("compile error")}}
	emitter := &mockEmitter{}
	space := &mockThinkSpace{}

	args := map[string]any{
		"agent_instructions": []any{"Initial instruction"},
	}
	flowCfg := flows.FlowConfig{RetryPrompt: "Trace: {{.ErrorTrace}}\nRules: {{.BaseAgentRules}}"}
	flowCtx := flows.FlowContext{FlowID: "flow-123", BaseAgentRules: "Must use /src dir"}

	var promptsReceived []string
	executor := func(ctx context.Context, instructions string, sandbox workspace.CandidateSandbox, agentID int, tokenChan chan<- workspace.AgentToken) error {
		promptsReceived = append(promptsReceived, instructions)
		return nil
	}

	_, _ = flow.Execute(context.Background(), service, thread, space, args, flowCfg, flowCtx, emitter, executor, verifier)

	if len(promptsReceived) != 2 {
		t.Fatalf("expected 2 execution attempts, got %d", len(promptsReceived))
	}

	retryPrompt := promptsReceived[1]
	if !strings.Contains(retryPrompt, "Trace: compile error") {
		t.Errorf("retry prompt missing error trace: %s", retryPrompt)
	}
	if !strings.Contains(retryPrompt, "Rules: Must use /src dir") {
		t.Errorf("retry prompt missing injected base rules: %s", retryPrompt)
	}
}

func TestFanOutFlow_DeliversOnFailure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	flow := flows.NewFanOutFlow(logger)

	engine := &mockChatEngine{}
	workspaceRoot := t.TempDir()
	service := workspace.NewService(logger, engine, workspaceRoot)
	thread, _ := service.StartThread(context.Background(), "chat-1")

	verifier := &mockVerifier{errsToReturn: []error{errors.New("err 1"), errors.New("err 2")}}
	emitter := &mockEmitter{}
	space := &mockThinkSpace{}

	args := map[string]any{
		"agent_instructions": []any{"Do a thing"},
	}
	flowCfg := flows.FlowConfig{RetryPrompt: "{{.ErrorTrace}}"}
	flowCtx := flows.FlowContext{FlowID: "flow-123"}

	executor := func(ctx context.Context, instructions string, sandbox workspace.CandidateSandbox, agentID int, tokenChan chan<- workspace.AgentToken) error {
		return nil
	}

	result, err := flow.Execute(context.Background(), service, thread, space, args, flowCfg, flowCtx, emitter, executor, verifier)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Branches) != 1 {
		t.Errorf("expected candidate to be returned despite failure")
	}
	if !engine.sandbox.delivered {
		t.Errorf("expected sandbox to be delivered even on failure")
	}

	lastEvent := emitter.events[len(emitter.events)-1]
	if lastEvent.Type != flows.FlowComplete {
		t.Fatalf("expected last event to be FlowComplete")
	}
	if lastEvent.Passed {
		t.Errorf("expected FlowComplete to report failure (Passed = false)")
	}
	if lastEvent.Trace != "err 2" {
		t.Errorf("expected final trace to contain 'err 2', got %q", lastEvent.Trace)
	}
}
