package flows_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/assembler"
	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"google.golang.org/genai"
)

// --- Mocks ---

type mockChatEngine struct {
	sandbox *mockSandbox
}

func (m *mockChatEngine) InitChat(ctx context.Context, chatID string) error {
	return nil
}

func (m *mockChatEngine) Snapshot(ctx context.Context, chatID string, msg string) (string, error) {
	return "", nil
}

func (m *mockChatEngine) PreviewCandidate(ctx context.Context, chatID string, candidateID string) error {
	return nil
}

func (m *mockChatEngine) ReadCandidateDiff(ctx context.Context, chatID string, candidateID string) (string, error) {
	return "+ mock candidate diff", nil
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

func (m *mockSandbox) WriteFile(ctx context.Context, path string, data []byte) error {
	return nil
}

func (m *mockSandbox) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if path == "trace.jsonl" {
		return []byte(`{"prompt": "Do a thing", "generated": "mock output"}`), nil
	}
	if path == "does/not/exist.go" {
		return nil, errors.New("file not found")
	}
	return []byte("mock physical file content"), nil
}

func (m *mockSandbox) ExecuteCommand(ctx context.Context, command string, args ...string) (string, error) {
	return "", nil
}

func (m *mockSandbox) ApplyDraft(ctx context.Context, message string) error {
	return nil
}

func (m *mockSandbox) DeliverForReview(ctx context.Context) error {
	m.delivered = true
	return nil
}

func (m *mockSandbox) TearDown(ctx context.Context) error {
	return nil
}

type mockThinkSpace struct{}

func (m *mockThinkSpace) Config() spaces.ThinkSpaceConfig {
	return spaces.ThinkSpaceConfig{}
}

func (m *mockThinkSpace) Tools() []*genai.Tool {
	return nil
}

func (m *mockThinkSpace) Verifier() spaces.Verifier {
	return nil
}

func (m *mockThinkSpace) Mapbook() assembler.Mapbook {
	return nil
}

func (m *mockThinkSpace) Patcher() workspace.Patcher {
	return nil
}

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

func TestFanOutFlow_CleanRun_WithJITInjection(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	flow := flows.NewFanOutFlow(logger)

	engine := &mockChatEngine{}
	workspaceRoot := t.TempDir()

	bus := chat.NewEventBus()
	service := workspace.NewService(logger, engine, workspaceRoot, bus)
	thread, _ := service.StartThread(context.Background(), "chat-1")

	verifier := &mockVerifier{}
	emitter := &mockEmitter{}
	var space spaces.ThinkSpace = &mockThinkSpace{}

	args := map[string]any{
		"agent_tasks": []any{
			map[string]any{
				"context_digest": "Basic server built",
				"instruction":    "Add auth",
				"target_files":   []any{"src/main.go"},
			},
		},
	}
	flowConfig := flows.FlowConfig{RetryPrompt: "Trace: {{.ErrorTrace}}"}
	flowContext := flows.FlowContext{FlowID: "flow-123", SpaceID: "golang"}

	var briefingsReceived []workspace.SubAgentBriefing
	executor := func(ctx context.Context, briefing workspace.SubAgentBriefing, sandbox workspace.CandidateSandbox, agentID int, tokenChan chan<- workspace.AgentToken) error {
		briefingsReceived = append(briefingsReceived, briefing)
		return nil
	}

	result, err := flow.Execute(context.Background(), service, thread, space, args, flowConfig, flowContext, emitter, executor, verifier)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(briefingsReceived) != 1 {
		t.Fatalf("expected 1 execution attempt, got %d", len(briefingsReceived))
	}

	if !strings.Contains(briefingsReceived[0].ContextDigest, "mock physical file content") {
		t.Errorf("expected JIT workbench content to be injected, got: %s", briefingsReceived[0].ContextDigest)
	}

	expectedBranch := "candidate/chat-1-flow-123-agent-1"
	if len(result.Branches) != 1 || result.Branches[0].CandidateID != expectedBranch {
		t.Errorf("expected %s, got %v", expectedBranch, result.Branches)
	}

	if !result.Branches[0].Passed {
		t.Errorf("expected branch to have passed verification")
	}

	if !engine.sandbox.delivered {
		t.Errorf("expected sandbox to be delivered")
	}
}

func TestFanOutFlow_FailFast_HallucinatedFile(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	flow := flows.NewFanOutFlow(logger)

	engine := &mockChatEngine{}
	workspaceRoot := t.TempDir()

	bus := chat.NewEventBus()
	service := workspace.NewService(logger, engine, workspaceRoot, bus)
	thread, _ := service.StartThread(context.Background(), "chat-1")

	verifier := &mockVerifier{}
	emitter := &mockEmitter{}
	var space spaces.ThinkSpace = &mockThinkSpace{}

	args := map[string]any{
		"agent_tasks": []any{
			map[string]any{
				"context_digest": "Testing Fail-Fast",
				"instruction":    "Edit the hallucinated file",
				"target_files":   []any{"does/not/exist.go"},
			},
		},
	}
	flowConfig := flows.FlowConfig{}
	flowContext := flows.FlowContext{FlowID: "flow-123", SpaceID: "golang"}

	executor := func(ctx context.Context, briefing workspace.SubAgentBriefing, sandbox workspace.CandidateSandbox, agentID int, tokenChan chan<- workspace.AgentToken) error {
		t.Fatal("executor should never be called; flow should fail-fast beforehand")
		return nil
	}

	_, err := flow.Execute(context.Background(), service, thread, space, args, flowConfig, flowContext, emitter, executor, verifier)

	if err == nil {
		t.Fatalf("expected fanout to fail due to fail-fast boundary, but it succeeded")
	}

	if !strings.Contains(err.Error(), "fatal system errors") {
		t.Errorf("expected fatal system error, got: %v", err)
	}
}

func TestFanOutFlow_RetryInjectsContext(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	flow := flows.NewFanOutFlow(logger)

	engine := &mockChatEngine{}
	workspaceRoot := t.TempDir()

	bus := chat.NewEventBus()
	service := workspace.NewService(logger, engine, workspaceRoot, bus)
	thread, _ := service.StartThread(context.Background(), "chat-1")

	verifier := &mockVerifier{errsToReturn: []error{errors.New("compile error")}}
	emitter := &mockEmitter{}
	var space spaces.ThinkSpace = &mockThinkSpace{}

	args := map[string]any{
		"agent_tasks": []any{
			map[string]any{
				"context_digest": "Basic context",
				"instruction":    "Initial instruction",
			},
		},
	}
	flowConfig := flows.FlowConfig{RetryPrompt: "Trace: {{.ErrorTrace}}\nRules: {{.BaseAgentRules}}"}
	flowContext := flows.FlowContext{FlowID: "flow-123", BaseAgentRules: "Must use /src dir"}

	var briefingsReceived []workspace.SubAgentBriefing
	executor := func(ctx context.Context, briefing workspace.SubAgentBriefing, sandbox workspace.CandidateSandbox, agentID int, tokenChan chan<- workspace.AgentToken) error {
		briefingsReceived = append(briefingsReceived, briefing)
		return nil
	}

	res, err := flow.Execute(context.Background(), service, thread, space, args, flowConfig, flowContext, emitter, executor, verifier)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(briefingsReceived) != 2 {
		t.Fatalf("expected 2 execution attempts, got %d", len(briefingsReceived))
	}

	retryInstruction := briefingsReceived[1].Instruction
	if !strings.Contains(retryInstruction, "Trace: compile error") {
		t.Errorf("retry instruction missing error trace: %s", retryInstruction)
	}

	if !res.Branches[0].Passed {
		t.Errorf("expected branch to have passed verification after retry")
	}
}
