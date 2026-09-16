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
