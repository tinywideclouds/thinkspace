package workspace_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type mockFileReader struct {
	files map[string][]byte
}

func (m *mockFileReader) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if data, ok := m.files[path]; ok {
		return data, nil
	}
	return nil, errors.New("file not found")
}

func TestReadFileContexts(t *testing.T) {
	ctx := context.Background()

	reader := &mockFileReader{
		files: map[string][]byte{
			"src/main.go":  []byte("package main"),
			"src/empty.go": []byte(""),
		},
	}

	t.Run("Empty Target Files", func(t *testing.T) {
		res, err := workspace.ReadFileContext(ctx, reader, []string{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(res) != 0 {
			t.Errorf("expected empty items slice, got %d items", len(res))
		}
	})

	t.Run("Success", func(t *testing.T) {
		res, err := workspace.ReadFileContext(ctx, reader, []string{"src/main.go"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 item, got %d", len(res))
		}
		if res[0].Path != "src/main.go" {
			t.Errorf("expected path 'src/main.go', got '%s'", res[0].Path)
		}
		if !strings.Contains(res[0].Content, "package main") {
			t.Errorf("missing file content in output: %s", res[0].Content)
		}
		if res[0].TokenCount != 3 {
			t.Errorf("expected TokenCount=3 for 12-byte file, got %d", res[0].TokenCount)
		}
	})

	t.Run("Allows Existing Empty File", func(t *testing.T) {
		res, err := workspace.ReadFileContext(ctx, reader, []string{"src/empty.go"})
		if err != nil {
			t.Fatalf("expected existing empty file to succeed, got: %v", err)
		}
		if len(res) != 1 || res[0].Content != "" || res[0].TokenCount != 0 {
			t.Errorf("expected 1 item with empty content and 0 tokens, got %+v", res)
		}
	})

	t.Run("Fail Fast on Missing File", func(t *testing.T) {
		_, err := workspace.ReadFileContext(ctx, reader, []string{"src/missing.go"})
		if err == nil {
			t.Errorf("expected fail-fast error on missing file")
		}
		if !strings.Contains(err.Error(), "fail-fast") {
			t.Errorf("expected fail-fast error, got: %v", err)
		}
	})
}
