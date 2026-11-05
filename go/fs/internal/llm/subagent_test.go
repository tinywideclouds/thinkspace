package llm_test

import (
	"context"
	"errors"
	"iter"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type mockSandbox struct {
	files map[string][]byte
}

func newMockSandbox() *mockSandbox {
	return &mockSandbox{files: make(map[string][]byte)}
}

func (m *mockSandbox) WriteFile(ctx context.Context, path string, data []byte) error {
	m.files[path] = data
	return nil
}

func (m *mockSandbox) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if data, ok := m.files[path]; ok {
		return data, nil
	}
	return nil, nil
}

func (m *mockSandbox) ExecuteCommand(ctx context.Context, command string, args ...string) (string, error) {
	return "", nil
}

func (m *mockSandbox) ApplyDraft(ctx context.Context, message string) error { return nil }
func (m *mockSandbox) DeliverForReview(ctx context.Context) error           { return nil }
func (m *mockSandbox) TearDown(ctx context.Context) error                   { return nil }

type spyModelClient struct {
	responses      []llm.StreamChunk
	err            error
	capturedModel  string
	capturedBundle cognition.ThinkBundle
	capturedTools  *tools.Registry
}

func (m *spyModelClient) GenerateContentStream(
	ctx context.Context,
	model string,
	bundle cognition.ThinkBundle,
	registry *tools.Registry,
	config spaces.ThinkStateConfig,
) iter.Seq2[llm.StreamChunk, error] {
	m.capturedModel = model
	m.capturedBundle = bundle
	m.capturedTools = registry

	return func(yield func(llm.StreamChunk, error) bool) {
		if m.err != nil {
			yield(llm.StreamChunk{}, m.err)
			return
		}
		for _, resp := range m.responses {
			if !yield(resp, nil) {
				return
			}
		}
	}
}

type mockPatcher struct {
	applyCalled bool
	lastOutput  string
	errToReturn error
}

func (m *mockPatcher) Apply(ctx context.Context, sandbox workspace.CandidateSandbox, llmOutput string) error {
	m.applyCalled = true
	m.lastOutput = llmOutput
	return m.errToReturn
}

func (m *mockPatcher) SystemInstructions() string {
	return "Mock patcher formatting rules"
}

func TestSubAgentFactory_ExecutionWithPatcher(t *testing.T) {
	ctx := context.Background()
	sandbox := newMockSandbox()
	patcher := &mockPatcher{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	domainSystemPrompt := "Base domain rules"
	taskInstruction := "write a main file"
	llmOutput := `{"patches": []}`

	spyClient := &spyModelClient{
		responses: []llm.StreamChunk{
			{Text: llmOutput},
		},
	}

	executor := llm.NewSubAgentExecutor(logger, spyClient, "test-worker", patcher)
	tokenChan := make(chan workspace.AgentToken, 10)

	bundle := cognition.ThinkBundle{
		Rules: cognition.SystemRules{
			Persona: domainSystemPrompt,
		},
		Trajectory: cognition.Trajectory{
			Catalyst: cognition.TrajectoryEvent{
				Role:    "user",
				Content: taskInstruction,
			},
		},
	}

	err := executor(ctx, bundle, sandbox, 1, tokenChan)
	if err != nil {
		t.Fatalf("executor failed: %v", err)
	}
	close(tokenChan)

	if spyClient.capturedTools != nil {
		t.Errorf("expected subagent to pass nil tool registry, but tools were provided")
	}

	if spyClient.capturedBundle.Rules.FormattingConstraints != "Mock patcher formatting rules" {
		t.Errorf("expected patcher instructions to be injected into bundle rules, got %q", spyClient.capturedBundle.Rules.FormattingConstraints)
	}

	if !patcher.applyCalled {
		t.Errorf("expected Patcher.Apply to be called")
	}
	if patcher.lastOutput != llmOutput {
		t.Errorf("expected patcher to receive raw output")
	}

	traceContent, exists := sandbox.files["trace.jsonl"]
	if !exists {
		t.Fatalf("expected trace.jsonl to be generated in sandbox")
	}
	if !strings.Contains(string(traceContent), taskInstruction) {
		t.Errorf("trace.jsonl missing prompt: %s", string(traceContent))
	}
}

func TestSubAgentFactory_EmptyResponse(t *testing.T) {
	ctx := context.Background()
	sandbox := newMockSandbox()
	patcher := &mockPatcher{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	spyClient := &spyModelClient{
		responses: []llm.StreamChunk{
			{Text: ""},
		},
	}

	executor := llm.NewSubAgentExecutor(logger, spyClient, "test-worker", patcher)

	err := executor(ctx, cognition.ThinkBundle{}, sandbox, 1, nil)
	if err == nil {
		t.Fatalf("expected error on empty response, got nil")
	}

	if !strings.Contains(err.Error(), "empty response") {
		t.Errorf("expected empty response error, got %v", err)
	}
}

func TestSubAgentFactory_PatcherFailure(t *testing.T) {
	ctx := context.Background()
	sandbox := newMockSandbox()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	patcher := &mockPatcher{
		errToReturn: errors.New("patching failed structurally"),
	}

	spyClient := &spyModelClient{
		responses: []llm.StreamChunk{
			{Text: "some output"},
		},
	}

	executor := llm.NewSubAgentExecutor(logger, spyClient, "test-worker", patcher)

	err := executor(ctx, cognition.ThinkBundle{}, sandbox, 1, nil)
	if err == nil {
		t.Fatalf("expected error bubbling from patcher, got nil")
	}

	if !strings.Contains(err.Error(), "patcher failed to apply changes") {
		t.Errorf("expected patcher error wrap, got %v", err)
	}
}
