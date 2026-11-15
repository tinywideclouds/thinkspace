package golang_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces/golang"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

// --- Mocks ---

type mockSandbox struct {
	files       map[string][]byte
	shouldDelay bool
}

func newMockSandbox() *mockSandbox {
	return &mockSandbox{
		files: make(map[string][]byte),
	}
}

func (m *mockSandbox) ResolvePath(path string) string {
	return path
}

func (m *mockSandbox) WriteFile(ctx context.Context, path string, data []byte) error {
	m.files[path] = data
	return nil
}

func (m *mockSandbox) ReadFile(ctx context.Context, path string) ([]byte, error) {
	data, ok := m.files[path]
	if !ok {
		return nil, fmt.Errorf("file not found")
	}
	return data, nil
}

func (m *mockSandbox) ExecuteCommand(ctx context.Context, command string, args ...string) (string, error) {
	cmdStr := command + " " + strings.Join(args, " ")

	if m.shouldDelay && strings.Contains(cmdStr, "go test") {
		select {
		case <-time.After(5 * time.Second):
			return "", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	if strings.Contains(cmdStr, "go build") {
		if mainCode, ok := m.files["src/main.go"]; ok {
			if strings.Contains(string(mainCode), "invalid") {
				return "syntax error", fmt.Errorf("exit status 1")
			}
		}
		if _, ok := m.files["src/go.mod"]; !ok {
			return "go.mod file not found", fmt.Errorf("exit status 1")
		}
		return "", nil
	}

	if strings.Contains(cmdStr, "go test") {
		if testCode, ok := m.files["src/main_test.go"]; ok {
			if strings.Contains(string(testCode), "fail") {
				return "test failed", fmt.Errorf("exit status 1")
			}
		}
		return "ok", nil
	}

	if strings.Contains(cmdStr, "go clean") {
		return "", nil
	}

	return "", fmt.Errorf("unknown command: %s", cmdStr)
}

func (m *mockSandbox) ApplyDraft(ctx context.Context, message string) error { return nil }
func (m *mockSandbox) DeliverForReview(ctx context.Context) error           { return nil }
func (m *mockSandbox) TearDown(ctx context.Context) error                   { return nil }

func setupGoThinkState(t *testing.T, verifyTimeout int) *golang.GoThinkState {
	config := spaces.ThinkStateConfig{
		Name: "golang",
		Options: map[string]any{
			"format_imports": true,
		},
		Toolchain: spaces.ToolchainConfig{
			Verify: []string{"go build -C src ./...", "go test -C src ./..."},
		},
		Timeouts: spaces.TimeoutsConfig{
			VerifySeconds: verifyTimeout,
		},
		Roles: spaces.RolesConfig{
			Manager: spaces.ManagerConfig{
				Tools: spaces.ToolsConfig{
					ProposeChange: spaces.ToolProposeChangeConfig{
						Description:              "mock tool desc",
						AgentCountDescription:    "mock count desc",
						ContextDigestDescription: "mock digest desc",
						InstructionDescription:   "mock instruction desc",
						TargetFilesDescription:   "mock files desc",
					},
					QueryLens: spaces.ToolQueryLensConfig{
						Description:          "mock query lens desc",
						TagDescription:       "mock lens tag",
						ReasoningDescription: "mock lens reasoning",
					},
				},
			},
		},
	}
	config.ApplyDefaults()
	return golang.NewGoThinkState(config)
}

// --- Verifier Tests ---

func TestCIVerifier_ASTError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	space := setupGoThinkState(t, 5)
	sandbox := newMockSandbox()

	sandbox.WriteFile(ctx, "src/go.mod", []byte("module test"))
	sandbox.WriteFile(ctx, "src/main.go", []byte("package main\nfunc invalid() {\n"))

	err := space.Verifier().Verify(ctx, sandbox)
	if err == nil {
		t.Fatalf("Expected AST verification to fail on invalid code")
	}
	if !strings.Contains(err.Error(), "syntax error") {
		t.Errorf("Expected error to contain output log, got: %v", err)
	}
}

func TestCIVerifier_MissingGoMod(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	space := setupGoThinkState(t, 5)
	sandbox := newMockSandbox()

	sandbox.WriteFile(ctx, "src/main.go", []byte("package main\nfunc main() {}\n"))

	err := space.Verifier().Verify(ctx, sandbox)
	if err == nil {
		t.Fatalf("Expected verification to fail due to missing go.mod")
	}
	if !strings.Contains(err.Error(), "go.mod file not found") {
		t.Errorf("Expected missing go.mod error, got: %v", err)
	}
}

func TestCIVerifier_Success(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	space := setupGoThinkState(t, 15)
	sandbox := newMockSandbox()

	sandbox.WriteFile(ctx, "src/go.mod", []byte("module testmod\n\ngo 1.21\n"))
	sandbox.WriteFile(ctx, "src/main.go", []byte("package main\nfunc main() {}\n"))
	sandbox.WriteFile(ctx, "src/main_test.go", []byte("package main\nimport \"testing\"\nfunc TestMain(t *testing.T) {}\n"))

	err := space.Verifier().Verify(ctx, sandbox)
	if err != nil {
		t.Fatalf("Expected successful verification, got: %v", err)
	}
}

func TestCIVerifier_TimeoutLoop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	space := setupGoThinkState(t, 1)
	sandbox := newMockSandbox()
	sandbox.shouldDelay = true

	sandbox.WriteFile(ctx, "src/go.mod", []byte("module testmod\n\ngo 1.21\n"))
	sandbox.WriteFile(ctx, "src/main.go", []byte("package main\nfunc main() {}\n"))

	err := space.Verifier().Verify(ctx, sandbox)
	if err == nil {
		t.Fatalf("Expected verification to fail due to timeout")
	}
	if !strings.Contains(err.Error(), "TIMEOUT") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

// --- Mapbook Tests ---
func TestGolangMapbook_GenerateLayers(t *testing.T) {
	ctx := context.Background()
	workspaceRoot := t.TempDir()

	srcDir := filepath.Join(workspaceRoot, "src")
	pkgDir := filepath.Join(srcDir, "api")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to setup test directories: %v", err)
	}

	_ = os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main"), 0644)
	_ = os.WriteFile(filepath.Join(pkgDir, "handler.go"), []byte("package api"), 0644)

	book := golang.NewGolangMapbook()
	layers, err := book.GenerateLayers(ctx, workspaceRoot)
	if err != nil {
		t.Fatalf("GenerateLayers failed: %v", err)
	}

	conceptual, ok := layers[spaces.LayerConceptual]
	if !ok || !strings.Contains(conceptual, "Standard Go project layout") {
		t.Errorf("missing or invalid conceptual layer")
	}

	structural, ok := layers[spaces.LayerStructural]
	if !ok {
		t.Fatalf("missing structural layer")
	}

	if !strings.Contains(structural, "src/main.go") {
		t.Errorf("structural tree missing src/main.go, got: \n%s", structural)
	}
	if !strings.Contains(structural, "src/api/handler.go") {
		t.Errorf("structural tree missing src/api/handler.go")
	}
}

func TestGolangMapbook_GenerateLayers_NoSrcDirectory(t *testing.T) {
	ctx := context.Background()
	workspaceRoot := t.TempDir()

	book := golang.NewGolangMapbook()
	layers, err := book.GenerateLayers(ctx, workspaceRoot)
	if err != nil {
		t.Fatalf("GenerateLayers should gracefully handle missing src directory, but got: %v", err)
	}

	structural, ok := layers[spaces.LayerStructural]
	if !ok {
		t.Fatalf("missing structural layer")
	}

	if strings.Contains(structural, "- src/") {
		t.Errorf("expected empty structural tree, got: %s", structural)
	}
}

// --- Interface Contract & Domain Option Tests ---
func TestGoThinkState_InterfaceContractsAndOptions(t *testing.T) {
	space := setupGoThinkState(t, 15)

	if space.Config().Name != "golang" {
		t.Errorf("Config() failed to return injected config")
	}
	if space.Verifier() == nil {
		t.Errorf("Verifier() returned nil")
	}
	if space.Mapbook() == nil {
		t.Errorf("Mapbook() returned nil")
	}
	if !space.Options().FormatImports {
		t.Errorf("expected FormatImports to be true")
	}

	// Verify disabling format_imports via domain options stops patcher from rewriting source
	disabledSpace := golang.NewGoThinkState(spaces.ThinkStateConfig{
		Name: "golang",
		Options: map[string]any{
			"format_imports": false,
		},
	})
	if disabledSpace.Options().FormatImports {
		t.Errorf("expected FormatImports to be false when disabled in Options")
	}

	sandbox := newMockSandbox()
	rawCode := "package main\nfunc main(){fmt.Println(\"hello\")}"
	payload, _ := json.Marshal(map[string]any{
		"patches": []map[string]string{
			{"file": "src/main.go", "action": "full_replace", "code": rawCode},
		},
	})
	if err := disabledSpace.Patcher().Apply(context.Background(), sandbox, string(payload)); err != nil {
		t.Fatalf("patcher failed: %v", err)
	}
	if string(sandbox.files["src/main.go"]) != rawCode {
		t.Errorf("expected raw code untouched when format_imports=false, got %q", string(sandbox.files["src/main.go"]))
	}
}

// --- Cognitive Hook Tests ---
func TestGoThinkState_EnrichBundle(t *testing.T) {
	ctx := context.Background()
	workspaceRoot := t.TempDir()

	srcDir := filepath.Join(workspaceRoot, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create src directory: %v", err)
	}

	expectedGoMod := "module thinkspace/app\n\ngo 1.27\n"
	if err := os.WriteFile(filepath.Join(srcDir, "go.mod"), []byte(expectedGoMod), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	space := setupGoThinkState(t, 15)

	t.Run("Proactively Loads Dependencies", func(t *testing.T) {
		bundle := &cognition.ThinkBundle{}
		focus := thinkgraph.AttentionFocus{
			PrimaryFocus: []string{"database", "dependencies"},
		}

		err := space.EnrichBundle(ctx, bundle, focus, workspaceRoot)
		if err != nil {
			t.Fatalf("EnrichBundle failed: %v", err)
		}

		if len(bundle.Workbench) != 1 {
			t.Fatalf("expected 1 item proactively loaded into workbench, got %d", len(bundle.Workbench))
		}

		item := bundle.Workbench[0]
		if item.Path != "src/go.mod" {
			t.Errorf("expected path 'src/go.mod', got '%s'", item.Path)
		}
		if item.Content != expectedGoMod {
			t.Errorf("expected content '%s', got '%s'", expectedGoMod, item.Content)
		}
	})

	t.Run("Ignores Unrelated Tags", func(t *testing.T) {
		bundle := &cognition.ThinkBundle{}
		focus := thinkgraph.AttentionFocus{
			PrimaryFocus: []string{"ui", "css"},
		}

		err := space.EnrichBundle(ctx, bundle, focus, workspaceRoot)
		if err != nil {
			t.Fatalf("EnrichBundle failed: %v", err)
		}

		if len(bundle.Workbench) != 0 {
			t.Errorf("expected 0 items in workbench, got %d", len(bundle.Workbench))
		}
	})
}
