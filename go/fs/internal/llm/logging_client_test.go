package llm_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
)

func TestLoggingModelClient_WritesTempFileAndLogsOnlyPointer(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))
	debugDir := t.TempDir()

	inner := &MockModelClient{
		Responses: []llm.StreamChunk{
			{Text: "chunk-1"},
			{Text: "chunk-2"},
		},
	}

	client := llm.NewLoggingModelClient(logger, debugDir, inner)

	bundle := cognition.ThinkBundle{
		Rules: cognition.SystemRules{
			Persona:               "Architect persona secret payload",
			FormattingConstraints: "Strict JSON",
		},
		Mapbook: cognition.Mapbook{
			Maps: map[string]string{
				"Structural": "src/main.go",
				"Conceptual": "Domain boundaries",
			},
		},
		Trajectory: cognition.Trajectory{
			LongTermMemory: []string{"Pinned invariant"},
			WorkingMemory: []cognition.TrajectoryEvent{
				{Role: "user", Content: "Previous prompt"},
				{Role: "model", Content: "Previous response"},
			},
			Catalyst: cognition.TrajectoryEvent{
				Role:    "user",
				Content: "Current instruction",
			},
		},
		Workbench: []cognition.WorkbenchItem{
			{Path: "src/main.go", Content: "package main\nfunc main() {}", TokenCount: 7},
		},
		ActiveTags: []string{"golang", "http"},
	}

	var received []string
	for chunk, err := range client.GenerateContentStream(context.Background(), "gemini-3.8-flash", bundle, tools.NewRegistry(), spaces.ThinkStateConfig{}) {
		if err != nil {
			t.Fatalf("unexpected stream error: %v", err)
		}
		received = append(received, chunk.Text)
	}

	if len(received) != 2 || received[0] != "chunk-1" || received[1] != "chunk-2" {
		t.Fatalf("expected chunks [chunk-1 chunk-2], got %v", received)
	}

	rawLog := logBuf.String()
	// Ensure heavy context payloads are NOT dumped into slog
	if strings.Contains(rawLog, "Architect persona secret payload") || strings.Contains(rawLog, "func main() {}") {
		t.Fatalf("expected slog output to contain only a file pointer, not full context payload: %s", rawLog)
	}

	var entry map[string]any
	if err := json.Unmarshal([]byte(rawLog), &entry); err != nil {
		t.Fatalf("failed decoding structured log entry %q: %v", rawLog, err)
	}

	if entry["msg"] != "Recorded LLM request context" {
		t.Errorf("expected log message 'Recorded LLM request context', got %v", entry["msg"])
	}
	if entry["model"] != "gemini-3.8-flash" {
		t.Errorf("expected model 'gemini-3.8-flash', got %v", entry["model"])
	}

	filePath, ok := entry["file"].(string)
	if !ok || filePath == "" {
		t.Fatalf("expected non-empty temp file path in log entry, got %v", entry["file"])
	}

	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed reading recorded LLM temp file %s: %v", filePath, err)
	}
	recorded := string(fileBytes)

	for _, want := range []string{
		"Architect persona secret payload",
		"Strict JSON",
		"Pinned invariant",
		"Previous prompt",
		"Previous response",
		"src/main.go",
		"package main\nfunc main() {}",
		"Current instruction",
	} {
		if !strings.Contains(recorded, want) {
			t.Errorf("expected recorded temp file to contain %q, got:\n%s", want, recorded)
		}
	}

	conceptualIdx := strings.Index(recorded, "### Layer: `Conceptual`")
	structuralIdx := strings.Index(recorded, "### Layer: `Structural`")
	if conceptualIdx == -1 || structuralIdx == -1 || conceptualIdx > structuralIdx {
		t.Errorf("expected sorted mapbook layers (Conceptual before Structural) in temp file")
	}
}

func TestLoggingModelClient_DefaultTempDirAndErrorPropagation(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

	expectedErr := errors.New("upstream rate limit")
	inner := &MockModelClient{Err: expectedErr}
	client := llm.NewLoggingModelClient(logger, "", inner)

	var gotErr error
	for _, err := range client.GenerateContentStream(context.Background(), "gemini-4-hydrogen", cognition.ThinkBundle{}, nil, spaces.ThinkStateConfig{}) {
		gotErr = err
	}
	if !errors.Is(gotErr, expectedErr) {
		t.Fatalf("expected propagated error %v, got %v", expectedErr, gotErr)
	}

	var entry map[string]any
	if err := json.Unmarshal(logBuf.Bytes(), &entry); err != nil {
		t.Fatalf("failed decoding structured log entry: %v", err)
	}
	if filePath, ok := entry["file"].(string); ok && filePath != "" {
		t.Cleanup(func() { _ = os.Remove(filePath) })
		if _, statErr := os.Stat(filePath); statErr != nil {
			t.Errorf("expected default temp file %s to exist: %v", filePath, statErr)
		}
	} else {
		t.Errorf("expected default temp file path to be logged")
	}

	nilClient := llm.NewLoggingModelClient(nil, "", nil)
	var nilErr error
	for _, err := range nilClient.GenerateContentStream(context.Background(), "gemini-4-hydrogen", cognition.ThinkBundle{}, nil, spaces.ThinkStateConfig{}) {
		nilErr = err
	}
	if nilErr == nil {
		t.Fatalf("expected error when inner ModelClient is nil")
	}
}
