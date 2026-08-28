package llm_test

import (
	"context"
	"strings"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"google.golang.org/genai"
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
	return nil, nil // Return nil safely if file doesn't exist yet
}

func (m *mockSandbox) ExecuteCommand(ctx context.Context, command string, args ...string) (string, error) {
	return "", nil
}

func (m *mockSandbox) ApplyDraft(ctx context.Context, message string) error { return nil }
func (m *mockSandbox) DeliverForReview(ctx context.Context) error           { return nil }
func (m *mockSandbox) TearDown(ctx context.Context) error                   { return nil }

func TestSubAgentFactory_Execution(t *testing.T) {
	ctx := context.Background()
	sandbox := newMockSandbox()

	jsonPayload := `{"main.go": "package main\n"}`

	mockClient := &MockModelClient{
		Responses: []*genai.GenerateContentResponse{
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

	executor := llm.SubAgentFactory(mockClient, "test-worker")
	tokenChan := make(chan workspace.AgentToken, 10)

	err := executor(ctx, "write a main file", sandbox, 1, tokenChan)
	if err != nil {
		t.Fatalf("executor failed: %v", err)
	}
	close(tokenChan)

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

	content, exists := sandbox.files["main.go"]
	if !exists {
		t.Fatalf("expected main.go to be generated in sandbox")
	}
	if string(content) != "package main\n" {
		t.Errorf("unexpected file content: %s", string(content))
	}

	traceContent, exists := sandbox.files["trace.jsonl"]
	if !exists {
		t.Fatalf("expected trace.jsonl to be generated in sandbox")
	}
	if !strings.Contains(string(traceContent), "write a main file") {
		t.Errorf("trace.jsonl missing prompt: %s", string(traceContent))
	}
	if !strings.Contains(string(traceContent), "main.go") {
		t.Errorf("trace.jsonl missing generated JSON: %s", string(traceContent))
	}
}

func TestSubAgentFactory_InvalidJSON(t *testing.T) {
	ctx := context.Background()
	sandbox := newMockSandbox()

	invalidPayload := `{"main.go": ` // missing closing brackets/quotes

	mockClient := &MockModelClient{
		Responses: []*genai.GenerateContentResponse{
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

	executor := llm.SubAgentFactory(mockClient, "test-worker")
	tokenChan := make(chan workspace.AgentToken, 10)

	err := executor(ctx, "write bad json", sandbox, 1, tokenChan)
	if err == nil {
		t.Fatalf("expected executor to fail on invalid json")
	}

	if !strings.Contains(err.Error(), "failed to parse sub-agent json") {
		t.Errorf("expected JSON parse error, got: %v", err)
	}
}
