package llm_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"google.golang.org/genai"
)

func TestSubAgentFactory_Execution(t *testing.T) {
	ctx := context.Background()
	sandboxDir := t.TempDir()

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

	err := executor(ctx, "write a main file", sandboxDir, 1, tokenChan)
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

	mainGoPath := filepath.Join(sandboxDir, "main.go")
	content, err := os.ReadFile(mainGoPath)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}
	if string(content) != "package main\n" {
		t.Errorf("unexpected file content: %s", string(content))
	}

	tracePath := filepath.Join(sandboxDir, "trace.jsonl")
	traceContent, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("failed to read trace.jsonl: %v", err)
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
	sandboxDir := t.TempDir()

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

	err := executor(ctx, "write bad json", sandboxDir, 1, tokenChan)
	if err == nil {
		t.Fatalf("expected executor to fail on invalid json")
	}

	if !strings.Contains(err.Error(), "failed to parse sub-agent json") {
		t.Errorf("expected JSON parse error, got: %v", err)
	}
}
