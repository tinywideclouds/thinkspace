package golang_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/spaces/golang"
)

type mockPatcherSandbox struct {
	rootDir string
	files   map[string][]byte
}

func newMockPatcherSandbox(rootDir string) *mockPatcherSandbox {
	return &mockPatcherSandbox{
		rootDir: rootDir,
		files:   make(map[string][]byte),
	}
}

func (m *mockPatcherSandbox) ResolvePath(path string) string {
	if m.rootDir == "" {
		return path
	}
	return filepath.Join(m.rootDir, filepath.FromSlash(path))
}

func (m *mockPatcherSandbox) WriteFile(ctx context.Context, path string, data []byte) error {
	m.files[path] = data
	if m.rootDir != "" {
		full := m.ResolvePath(path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			return err
		}
		return os.WriteFile(full, data, 0644)
	}
	return nil
}

func (m *mockPatcherSandbox) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if data, ok := m.files[path]; ok {
		return data, nil
	}
	if m.rootDir != "" {
		return os.ReadFile(m.ResolvePath(path))
	}
	return nil, os.ErrNotExist
}

func (m *mockPatcherSandbox) ExecuteCommand(ctx context.Context, command string, args ...string) (string, error) {
	return "", nil
}

func (m *mockPatcherSandbox) ApplyDraft(ctx context.Context, message string) error { return nil }
func (m *mockPatcherSandbox) DeliverForReview(ctx context.Context) error           { return nil }
func (m *mockPatcherSandbox) TearDown(ctx context.Context) error                   { return nil }

func TestGoASTPatcher_InvalidJSON(t *testing.T) {
	ctx := context.Background()
	sandbox := newMockPatcherSandbox("")
	patcher := golang.NewGoASTPatcher("mock instructions", true)

	invalidPayload := `{"patches": [{"file": "main.go", `

	err := patcher.Apply(ctx, sandbox, invalidPayload)
	if err == nil {
		t.Fatalf("expected patcher to fail on fatally invalid json")
	}

	if !strings.Contains(err.Error(), "invalid patch JSON") {
		t.Errorf("expected JSON parse error, got: %v", err)
	}
}

func TestGoASTPatcher_HealsBackslashSpaceAndMarkdownFences(t *testing.T) {
	ctx := context.Background()
	sandbox := newMockPatcherSandbox(t.TempDir())
	patcher := golang.NewGoASTPatcher("mock instructions", false)

	rawOutput := "```json\n{\"thought\":\"Applying fix\",\"patches\":[{\"file\":\"src/main.go\",\"action\":\"full_replace\",\"code\":\"package main\\n// hello\\ world\"}]}\n```"
	if err := patcher.Apply(ctx, sandbox, rawOutput); err != nil {
		t.Fatalf("expected patcher to heal backslash-space escape and markdown fences, got: %v", err)
	}

	got := string(sandbox.files["src/main.go"])
	if !strings.Contains(got, "// hello world") {
		t.Errorf("expected healed Go source content, got:\n%s", got)
	}
}

func TestGoASTPatcher_FormatImports_AddsMissingImportsAndFormats(t *testing.T) {
	ctx := context.Background()
	sandboxRoot := t.TempDir()
	sandbox := newMockPatcherSandbox(sandboxRoot)

	if err := sandbox.WriteFile(ctx, "src/go.mod", []byte("module testapp\n\ngo 1.24\n")); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	patcher := golang.NewGoASTPatcher("mock instructions", true)
	payloadBytes, err := json.Marshal(map[string]any{
		"thought": "Write unformatted main without imports",
		"patches": []map[string]string{
			{
				"file":   "src/main.go",
				"action": "full_replace",
				"code":   "package main\nfunc main(){\nfmt.Println(\"hello\")\n}",
			},
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	if err := patcher.Apply(ctx, sandbox, string(payloadBytes)); err != nil {
		t.Fatalf("unexpected patcher error: %v", err)
	}

	result := string(sandbox.files["src/main.go"])
	if !strings.Contains(result, `import "fmt"`) {
		t.Errorf("expected goimports to inject missing \"fmt\" import, got:\n%s", result)
	}
	if !strings.Contains(result, "func main() {\n\tfmt.Println(\"hello\")\n}") {
		t.Errorf("expected goimports to format function block, got:\n%s", result)
	}
}

func TestGoASTPatcher_FormatImports_DisabledLeavesSourceUntouched(t *testing.T) {
	ctx := context.Background()
	sandbox := newMockPatcherSandbox(t.TempDir())
	patcher := golang.NewGoASTPatcher("mock instructions", false)

	rawCode := "package main\nfunc main(){fmt.Println(\"hello\")}"
	payloadBytes, err := json.Marshal(map[string]any{
		"thought": "Write raw code",
		"patches": []map[string]string{
			{
				"file":   "src/main.go",
				"action": "full_replace",
				"code":   rawCode,
			},
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	if err := patcher.Apply(ctx, sandbox, string(payloadBytes)); err != nil {
		t.Fatalf("unexpected patcher error: %v", err)
	}

	if string(sandbox.files["src/main.go"]) != rawCode {
		t.Errorf("expected raw code to remain untouched when formatImports is false, got:\n%s", string(sandbox.files["src/main.go"]))
	}
}

func TestGoASTPatcher_FormatImports_SyntaxErrorFallsBackToRaw(t *testing.T) {
	ctx := context.Background()
	sandbox := newMockPatcherSandbox(t.TempDir())
	patcher := golang.NewGoASTPatcher("mock instructions", true)

	brokenCode := "package main\nfunc broken( {\n"
	payloadBytes, err := json.Marshal(map[string]any{
		"thought": "Broken syntax",
		"patches": []map[string]string{
			{
				"file":   "src/main.go",
				"action": "full_replace",
				"code":   brokenCode,
			},
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	if err := patcher.Apply(ctx, sandbox, string(payloadBytes)); err != nil {
		t.Fatalf("expected patcher to fall back gracefully on syntax error, got: %v", err)
	}

	if string(sandbox.files["src/main.go"]) != brokenCode {
		t.Errorf("expected broken code to be written raw for compiler verification, got:\n%s", string(sandbox.files["src/main.go"]))
	}
}

func TestGoASTPatcher_ReplaceFunction_UpdatesImports(t *testing.T) {
	ctx := context.Background()
	sandbox := newMockPatcherSandbox(t.TempDir())
	if err := sandbox.WriteFile(ctx, "src/go.mod", []byte("module testapp\n\ngo 1.24\n")); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := sandbox.WriteFile(ctx, "src/main.go", []byte("package main\n\nimport \"strings\"\n\nfunc greet() string {\n\treturn strings.ToUpper(\"hi\")\n}\n")); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	patcher := golang.NewGoASTPatcher("mock instructions", true)
	payloadBytes, err := json.Marshal(map[string]any{
		"thought": "Switch strings to fmt",
		"patches": []map[string]string{
			{
				"file":   "src/main.go",
				"action": "replace_function",
				"name":   "greet",
				"code":   "func greet() string {\n\treturn fmt.Sprintf(\"hi %d\", 1)\n}",
			},
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	if err := patcher.Apply(ctx, sandbox, string(payloadBytes)); err != nil {
		t.Fatalf("unexpected patcher error: %v", err)
	}

	result := string(sandbox.files["src/main.go"])
	if !strings.Contains(result, `import "fmt"`) {
		t.Errorf("expected \"fmt\" import to be added after replace_function, got:\n%s", result)
	}
	if strings.Contains(result, `"strings"`) {
		t.Errorf("expected unused \"strings\" import to be pruned after replace_function, got:\n%s", result)
	}
}

func TestGoASTPatcher_AddCodeAndValidationErrors(t *testing.T) {
	ctx := context.Background()
	sandbox := newMockPatcherSandbox(t.TempDir())
	if err := sandbox.WriteFile(ctx, "src/main.go", []byte("package main\n")); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	patcher := golang.NewGoASTPatcher("mock instructions", false)

	// add_code appends cleanly
	addPayload, _ := json.Marshal(map[string]any{
		"patches": []map[string]string{
			{"file": "src/main.go", "action": "add_code", "code": "func helper() {}"},
		},
	})
	if err := patcher.Apply(ctx, sandbox, string(addPayload)); err != nil {
		t.Fatalf("add_code failed: %v", err)
	}
	if !strings.Contains(string(sandbox.files["src/main.go"]), "func helper() {}") {
		t.Errorf("expected helper() to be appended, got:\n%s", string(sandbox.files["src/main.go"]))
	}

	// Unsupported action must return an error instead of silently succeeding
	unknownActionPayload, _ := json.Marshal(map[string]any{
		"patches": []map[string]string{
			{"file": "src/main.go", "action": "delete_everything", "code": ""},
		},
	})
	if err := patcher.Apply(ctx, sandbox, string(unknownActionPayload)); err == nil {
		t.Errorf("expected error for unsupported patch action")
	}

	// Escaping path must return an error
	traversalPayload, _ := json.Marshal(map[string]string{
		"file": "../escape.go", "action": "full_replace", "code": "package main",
	})
	if err := patcher.Apply(ctx, sandbox, `{"patches":[`+string(traversalPayload)+`]}`); err == nil {
		t.Errorf("expected error for path traversal in patch file")
	}
}
