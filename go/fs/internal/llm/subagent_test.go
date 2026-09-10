package llm_test

import (
	"context"
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
	return nil, nil // Return nil safely if file doesn't exist yet
}

func (m *mockSandbox) ExecuteCommand(ctx context.Context, command string, args ...string) (string, error) {
	return "", nil
}

func (m *mockSandbox) ApplyDraft(ctx context.Context, message string) error { return nil }
func (m *mockSandbox) DeliverForReview(ctx context.Context) error           { return nil }
func (m *mockSandbox) TearDown(ctx context.Context) error                   { return nil }

// spyModelClient implements ModelClient and captures the arguments passed to GenerateContentStream.
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

// --- Tests ---

func TestSubAgentFactory_Execution(t *testing.T) {
	ctx := context.Background()
	sandbox := newMockSandbox()

	domainSystemPrompt := "You must place all Go files in the src/ directory."
	taskInstruction := "write a main file"
	jsonPayload := `{"src/main.go": "package main\n"}`
	maxTokens := 8192

	spyClient := &spyModelClient{
		responses: []*genai.GenerateContentResponse{
			{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []*genai.Part{
								{Text: jsonPayload},
							},
						},
					},
				},
			},
		},
	}

	executor := llm.NewSubAgentExecutor(spyClient, "test-worker", domainSystemPrompt, maxTokens)
	tokenChan := make(chan workspace.AgentToken, 10)

	briefing := workspace.SubAgentBriefing{Instruction: taskInstruction}
	err := executor(ctx, briefing, sandbox, 1, tokenChan)
	if err != nil {
		t.Fatalf("executor failed: %v", err)
	}
	close(tokenChan)

	// 1. Assert the AI was configured with the domain's System Prompt
	if spyClient.capturedConfig == nil || spyClient.capturedConfig.SystemInstruction == nil {
		t.Fatalf("expected SystemInstruction to be populated in the SDK config")
	}
	actualSysPrompt := spyClient.capturedConfig.SystemInstruction.Parts[0].Text
	if actualSysPrompt != domainSystemPrompt {
		t.Errorf("expected system prompt %q, got %q", domainSystemPrompt, actualSysPrompt)
	}

	// 2. Assert Max Tokens were applied using direct int32 comparison
	if spyClient.capturedConfig.MaxOutputTokens != int32(maxTokens) {
		t.Errorf("expected MaxOutputTokens to be set to %d, got %d", maxTokens, spyClient.capturedConfig.MaxOutputTokens)
	}

	// 3. Assert the AI was passed the specific agent instructions
	if len(spyClient.capturedHistory) == 0 {
		t.Fatalf("expected history to contain the task instructions")
	}
	actualInstruction := spyClient.capturedHistory[0].Parts[0].Text
	if !strings.Contains(actualInstruction, taskInstruction) {
		t.Errorf("expected task instruction %q to be in %q", taskInstruction, actualInstruction)
	}

	// 4. Assert the token stream multiplexing works
	var streamedText strings.Builder
	for token := range tokenChan {
		if token.AgentID != 1 {
			t.Errorf("expected agent ID 1, got %d", token.AgentID)
		}
		streamedText.WriteString(token.Text)
	}

	if streamedText.String() != jsonPayload {
		t.Errorf("expected streamed text to be '%s', got '%s'", jsonPayload, streamedText.String())
	}

	// 5. Assert the physical sandbox correctly interpreted the JSON payload
	content, exists := sandbox.files["src/main.go"]
	if !exists {
		t.Fatalf("expected src/main.go to be generated in sandbox")
	}
	if string(content) != "package main\n" {
		t.Errorf("unexpected file content: %s", string(content))
	}

	// 6. Assert the trace ledger logged the execution accurately
	traceContent, exists := sandbox.files["trace.jsonl"]
	if !exists {
		t.Fatalf("expected trace.jsonl to be generated in sandbox")
	}
	if !strings.Contains(string(traceContent), taskInstruction) {
		t.Errorf("trace.jsonl missing prompt: %s", string(traceContent))
	}
	if !strings.Contains(string(traceContent), "src/main.go") {
		t.Errorf("trace.jsonl missing generated JSON: %s", string(traceContent))
	}
}

func TestSubAgentFactory_RecoverableJSON(t *testing.T) {
	ctx := context.Background()
	sandbox := newMockSandbox()

	// This string contains an invalid escape sequence `\)` which LLMs occasionally hallucinate
	recoverablePayload := `{"src/server.go": "import (\n\t\"net/http\"\)\n"}`

	spyClient := &spyModelClient{
		responses: []*genai.GenerateContentResponse{
			{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []*genai.Part{
								{Text: recoverablePayload},
							},
						},
					},
				},
			},
		},
	}

	executor := llm.NewSubAgentExecutor(spyClient, "test-worker", "Mock system instructions", 8192)
	tokenChan := make(chan workspace.AgentToken, 10)

	briefing := workspace.SubAgentBriefing{Instruction: "write server with a bad escape"}

	// Execute should succeed because the regex sanitizer fixes the broken `\)`
	err := executor(ctx, briefing, sandbox, 1, tokenChan)
	if err != nil {
		t.Fatalf("expected executor to recover from bad escapes, but failed: %v", err)
	}

	content, exists := sandbox.files["src/server.go"]
	if !exists {
		t.Fatalf("expected src/server.go to be generated in sandbox")
	}

	// The trailing `\)` should have been cleanly sanitized to `)`
	expectedContent := "import (\n\t\"net/http\")\n"
	if string(content) != expectedContent {
		t.Errorf("expected file content:\n%s\ngot:\n%s", expectedContent, string(content))
	}
}

func TestSubAgentFactory_InvalidJSON(t *testing.T) {
	ctx := context.Background()
	sandbox := newMockSandbox()

	invalidPayload := `{"main.go": ` // missing closing brackets/quotes. Unrecoverable.

	spyClient := &spyModelClient{
		responses: []*genai.GenerateContentResponse{
			{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []*genai.Part{
								{Text: invalidPayload},
							},
						},
					},
				},
			},
		},
	}

	executor := llm.NewSubAgentExecutor(spyClient, "test-worker", "Mock system instructions", 8192)
	tokenChan := make(chan workspace.AgentToken, 10)

	briefing := workspace.SubAgentBriefing{Instruction: "write bad json"}
	err := executor(ctx, briefing, sandbox, 1, tokenChan)
	if err == nil {
		t.Fatalf("expected executor to fail on fatally invalid json")
	}

	if !strings.Contains(err.Error(), "failed to parse sub-agent json") {
		t.Errorf("expected JSON parse error, got: %v", err)
	}
}
