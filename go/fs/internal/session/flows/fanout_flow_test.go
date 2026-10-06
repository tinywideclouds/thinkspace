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
	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
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

func (m *mockThinkSpace) Tools() []llm.ToolDef {
	return nil
}

func (m *mockThinkSpace) Verifier() spaces.Verifier {
	return nil
}

type mockMapbook struct{}

func (m *mockMapbook) GenerateLayers(ctx context.Context, workspaceRoot string) (map[assembler.MapbookLayer]string, error) {
	return map[assembler.MapbookLayer]string{
		assembler.LayerStructural: "Mock Tree",
	}, nil
}

func (m *mockThinkSpace) Mapbook() assembler.Mapbook {
	return &mockMapbook{}
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

type emittedEvent struct {
	channel string
	event   telemetry.Event
}

type mockEmitter struct {
	events []emittedEvent
}

func (m *mockEmitter) Emit(channel string, event telemetry.Event) {
	m.events = append(m.events, emittedEvent{channel: channel, event: event})
}

// --- Tests ---

func TestFanOutFlow_CleanRun_WithThinkBundle(t *testing.T) {
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
	flowContext := flows.FlowContext{FlowID: "flow-123", SpaceID: "golang", BaseAgentRules: "Worker Persona"}

	var bundlesReceived []cognition.ThinkBundle
	executor := func(ctx context.Context, bundle cognition.ThinkBundle, sandbox workspace.CandidateSandbox, agentID int, tokenChan chan<- workspace.AgentToken) error {
		bundlesReceived = append(bundlesReceived, bundle)
		return nil
	}

	result, err := flow.Execute(context.Background(), service, thread, space, args, flowConfig, flowContext, emitter, executor, verifier)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(bundlesReceived) != 1 {
		t.Fatalf("expected 1 execution attempt, got %d", len(bundlesReceived))
	}

	bundle := bundlesReceived[0]
	if bundle.Rules.Persona != "Worker Persona" {
		t.Errorf("expected Worker Persona in rules")
	}
	if len(bundle.Workbench) != 1 || bundle.Workbench[0].Path != "src/main.go" {
		t.Errorf("expected Workbench to materialize target file")
	}
	if len(bundle.Trajectory.LongTermMemory) != 1 || bundle.Trajectory.LongTermMemory[0] != "Basic server built" {
		t.Errorf("expected context digest in long term memory")
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

	executor := func(ctx context.Context, bundle cognition.ThinkBundle, sandbox workspace.CandidateSandbox, agentID int, tokenChan chan<- workspace.AgentToken) error {
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

	var bundlesReceived []cognition.ThinkBundle
	executor := func(ctx context.Context, bundle cognition.ThinkBundle, sandbox workspace.CandidateSandbox, agentID int, tokenChan chan<- workspace.AgentToken) error {
		bundlesReceived = append(bundlesReceived, bundle)
		return nil
	}

	res, err := flow.Execute(context.Background(), service, thread, space, args, flowConfig, flowContext, emitter, executor, verifier)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(bundlesReceived) != 2 {
		t.Fatalf("expected 2 execution attempts, got %d", len(bundlesReceived))
	}

	retryInstruction := bundlesReceived[1].Trajectory.Catalyst.Content
	if !strings.Contains(retryInstruction, "Trace: compile error") {
		t.Errorf("retry instruction missing error trace: %s", retryInstruction)
	}

	if !res.Branches[0].Passed {
		t.Errorf("expected branch to have passed verification after retry")
	}
}

func TestFanOutFlow_TelemetryEmission(t *testing.T) {
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
				"context_digest": "Context",
				"instruction":    "Instruction",
			},
		},
	}
	flowConfig := flows.FlowConfig{}
	flowContext := flows.FlowContext{FlowID: "flow-999", SpaceID: "golang", FlowSummary: "Testing telemetry routing"}

	executor := func(ctx context.Context, bundle cognition.ThinkBundle, sandbox workspace.CandidateSandbox, agentID int, tokenChan chan<- workspace.AgentToken) error {
		return nil
	}

	_, _ = flow.Execute(context.Background(), service, thread, space, args, flowConfig, flowContext, emitter, executor, verifier)

	var foundStart, foundSpawn, foundComplete bool

	for _, e := range emitter.events {
		if e.channel == "manager" {
			if e.event.Type == "flow_start" {
				foundStart = true
				if e.event.Summary != "Testing telemetry routing" {
					t.Errorf("expected flow_start to carry the FlowSummary, got: %s", e.event.Summary)
				}
				if e.event.Payload["agent_count"].(int) != 1 {
					t.Errorf("expected payload to have agent_count = 1")
				}
			}
		}

		if e.channel == "agent-1" {
			if e.event.Type == "flow_spawn" {
				foundSpawn = true
				if e.event.Payload["instruction"] != "Instruction" {
					t.Errorf("expected flow_spawn payload to carry instruction")
				}
			}
		}

		if e.channel == "manager" {
			if e.event.Type == "flow_complete" {
				foundComplete = true
				if e.event.Payload["passed"] != true {
					t.Errorf("expected flow_complete payload to reflect pass state")
				}
			}
		}
	}

	if !foundStart {
		t.Errorf("failed to emit flow_start to 'manager' channel")
	}
	if !foundSpawn {
		t.Errorf("failed to emit flow_spawn to 'agent-1' channel")
	}
	if !foundComplete {
		t.Errorf("failed to emit flow_complete back to 'manager' channel")
	}
}
