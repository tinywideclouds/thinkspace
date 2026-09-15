package golang_test

import (
	"context"
	"strings"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/spaces/golang"
)

type mockPatcherSandbox struct {
	files map[string][]byte
}

func newMockPatcherSandbox() *mockPatcherSandbox {
	return &mockPatcherSandbox{files: make(map[string][]byte)}
}

func (m *mockPatcherSandbox) WriteFile(ctx context.Context, path string, data []byte) error {
	m.files[path] = data
	return nil
}

func (m *mockPatcherSandbox) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if data, ok := m.files[path]; ok {
		return data, nil
	}
	return nil, nil
}

func (m *mockPatcherSandbox) ExecuteCommand(ctx context.Context, command string, args ...string) (string, error) {
	return "", nil
}

func (m *mockPatcherSandbox) ApplyDraft(ctx context.Context, message string) error { return nil }
func (m *mockPatcherSandbox) DeliverForReview(ctx context.Context) error           { return nil }
func (m *mockPatcherSandbox) TearDown(ctx context.Context) error                   { return nil }

func TestGoASTPatcher_RecoverableJSON(t *testing.T) {
	ctx := context.Background()
	sandbox := newMockPatcherSandbox()
	patcher := golang.NewGoASTPatcher("mock instructions")

	// This string contains an invalid escape sequence `\)` which LLMs occasionally hallucinate
	recoverablePayload := `{"patches": [{"file": "src/server.go", "action": "full_replace", "code": "import (\n\t\"net/http\"\)\n"}]}`

	err := patcher.Apply(ctx, sandbox, recoverablePayload)
	if err != nil {
		t.Fatalf("expected patcher to recover from bad escapes, but failed: %v", err)
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

func TestGoASTPatcher_InvalidJSON(t *testing.T) {
	ctx := context.Background()
	sandbox := newMockPatcherSandbox()
	patcher := golang.NewGoASTPatcher("mock instructions")

	invalidPayload := `{"patches": [{"file": "main.go", ` // missing closing brackets/quotes

	err := patcher.Apply(ctx, sandbox, invalidPayload)
	if err == nil {
		t.Fatalf("expected patcher to fail on fatally invalid json")
	}

	if !strings.Contains(err.Error(), "invalid patch JSON") {
		t.Errorf("expected JSON parse error, got: %v", err)
	}
}

func TestGoASTPatcher_ExtractsMarkdownJSON(t *testing.T) {
	ctx := context.Background()
	sandbox := newMockPatcherSandbox()
	patcher := golang.NewGoASTPatcher("mock instructions")

	// LLMs often wrap JSON in markdown blocks despite instructions
	markdownPayload := "Here is the code:\n```json\n{\"patches\": [{\"file\": \"src/main.go\", \"action\": \"full_replace\", \"code\": \"package main\"}]}\n```"

	err := patcher.Apply(ctx, sandbox, markdownPayload)
	if err != nil {
		t.Fatalf("expected patcher to extract JSON from markdown, but failed: %v", err)
	}

	if string(sandbox.files["src/main.go"]) != "package main" {
		t.Errorf("failed to apply extracted code")
	}
}
