package assembler_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/assembler"
)

type mockWorkbenchSandbox struct {
	files map[string][]byte
}

func (m *mockWorkbenchSandbox) WriteFile(ctx context.Context, path string, data []byte) error {
	return nil
}

func (m *mockWorkbenchSandbox) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if data, ok := m.files[path]; ok {
		return data, nil
	}
	return nil, errors.New("file not found")
}

func (m *mockWorkbenchSandbox) ExecuteCommand(ctx context.Context, cmd string, args ...string) (string, error) {
	return "", nil
}

func (m *mockWorkbenchSandbox) ApplyDraft(ctx context.Context, msg string) error { return nil }
func (m *mockWorkbenchSandbox) DeliverForReview(ctx context.Context) error       { return nil }
func (m *mockWorkbenchSandbox) TearDown(ctx context.Context) error               { return nil }

func TestWorkbench_Build(t *testing.T) {
	ctx := context.Background()
	workbench := assembler.NewWorkbench()

	sandbox := &mockWorkbenchSandbox{
		files: map[string][]byte{
			"src/main.go": []byte("package main"),
		},
	}

	t.Run("Empty Target Files", func(t *testing.T) {
		res, err := workbench.Build(ctx, sandbox, []string{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if res != "" {
			t.Errorf("expected empty string, got %s", res)
		}
	})

	t.Run("Success", func(t *testing.T) {
		res, err := workbench.Build(ctx, sandbox, []string{"src/main.go"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !strings.Contains(res, "package main") {
			t.Errorf("missing file content in output: %s", res)
		}
		if !strings.Contains(res, "### Workbench Files") {
			t.Errorf("missing markdown header in output")
		}
	})

	t.Run("Fail Fast on Missing File", func(t *testing.T) {
		_, err := workbench.Build(ctx, sandbox, []string{"src/missing.go"})
		if err == nil {
			t.Errorf("expected fail-fast error on missing file")
		}
		if !strings.Contains(err.Error(), "fail-fast") {
			t.Errorf("expected fail-fast error, got: %v", err)
		}
	})
}
