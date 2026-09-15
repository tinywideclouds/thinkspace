package llm_test

import (
	"context"
	"errors"
	"iter"
	"strings"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"google.golang.org/genai"
)

// --- Mocks ---

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
	responses       []*genai.GenerateContentResponse
	err             error
	capturedModel   string
	capturedHistory []*genai.Content
	capturedConfig  *genai.GenerateContentConfig
}

func (m *spyModelClient) GenerateContentStream(ctx context.Context, model string, history []*genai.Content, config *genai.GenerateContentConfig) iter.Seq2[*genai.GenerateContentResponse, error] {
	m.capturedModel = model
	m.capturedHistory = history
	m.capturedConfig = config

	return func(yield func(*genai.GenerateContentResponse, error) bool) {
		if m.err != nil {
			yield(nil, m.err)
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

// --- Tests ---

func TestSubAgentFactory_ExecutionWithPatcher(t *testing.T) {
	ctx := context.Background()
	sandbox := newMockSandbox()
	patcher := &mockPatcher{}

	domainSystemPrompt := "Base domain rules"
	taskInstruction := "write a main file"
	llmOutput := `{"patches": []}`
	maxTokens := 8192

	spyClient := &spyModelClient{
		responses: []*genai.GenerateContentResponse{
			{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []*genai.Part{{Text: llmOutput}},
						},
					},
				},
			},
		},
	}

	executor := llm.NewSubAgentExecutor(spyClient, "test-worker", domainSystemPrompt, maxTokens, patcher)
	tokenChan := make(chan workspace.AgentToken, 10)

	briefing := workspace.SubAgentBriefing{Instruction: taskInstruction}
	err := executor(ctx, briefing, sandbox, 1, tokenChan)
	if err != nil {
		t.Fatalf("executor failed: %v", err)
	}
	close(tokenChan)

	// Assert Instructions Fused
	actualSysPrompt := spyClient.capturedConfig.SystemInstruction.Parts[0].Text
	if !strings.Contains(actualSysPrompt, "Base domain rules") || !strings.Contains(actualSysPrompt, "Mock patcher formatting rules") {
		t.Errorf("expected combined system prompt, got %q", actualSysPrompt)
	}

	// Assert Patcher Call
	if !patcher.applyCalled {
		t.Errorf("expected Patcher.Apply to be called")
	}
	if patcher.lastOutput != llmOutput {
		t.Errorf("expected patcher to receive raw output")
	}

	// Assert Trace logging
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

	spyClient := &spyModelClient{
		responses: []*genai.GenerateContentResponse{
			{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []*genai.Part{{Text: ""}},
						},
					},
				},
			},
		},
	}

	executor := llm.NewSubAgentExecutor(spyClient, "test-worker", "rules", 100, patcher)

	err := executor(ctx, workspace.SubAgentBriefing{Instruction: "do work"}, sandbox, 1, nil)
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
	patcher := &mockPatcher{
		errToReturn: errors.New("patching failed structurally"),
	}

	spyClient := &spyModelClient{
		responses: []*genai.GenerateContentResponse{
			{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []*genai.Part{{Text: "some output"}},
						},
					},
				},
			},
		},
	}

	executor := llm.NewSubAgentExecutor(spyClient, "test-worker", "rules", 100, patcher)

	err := executor(ctx, workspace.SubAgentBriefing{Instruction: "do work"}, sandbox, 1, nil)
	if err == nil {
		t.Fatalf("expected error bubbling from patcher, got nil")
	}

	if !strings.Contains(err.Error(), "patcher failed to apply changes") {
		t.Errorf("expected patcher error wrap, got %v", err)
	}
}
