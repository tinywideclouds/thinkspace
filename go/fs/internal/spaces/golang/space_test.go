package golang_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/assembler"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces/golang"
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

	return "", fmt.Errorf("unknown command: %s", cmdStr)
}

func (m *mockSandbox) ApplyDraft(ctx context.Context, message string) error { return nil }
func (m *mockSandbox) DeliverForReview(ctx context.Context) error           { return nil }
func (m *mockSandbox) TearDown(ctx context.Context) error                   { return nil }

func setupGoThinkSpace(t *testing.T, verifyTimeout int) *golang.GoThinkSpace {
	config := spaces.ThinkSpaceConfig{
		Name:                         "golang",
		VerifyTimeoutSeconds:         verifyTimeout,
		ToolDescription:              "mock tool desc",
		AgentCountDescription:        "mock count desc",
		AssignedTagsDescription:      "mock tags desc",
		AgentInstructionsDescription: "mock instructions desc",
		ContextDigestDescription:     "mock digest desc",
		InstructionDescription:       "mock instruction desc",
		TargetFilesDescription:       "mock files desc",
	}
	config.ApplyDefaults()
	return golang.NewGoThinkSpace(config)
}

// --- Verifier Tests ---

func TestGoThinkSpace_Verify_ASTError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	space := setupGoThinkSpace(t, 5)
	sandbox := newMockSandbox()

	sandbox.WriteFile(ctx, "src/go.mod", []byte("module test"))
	sandbox.WriteFile(ctx, "src/main.go", []byte("package main\nfunc invalid() {\n"))

	err := space.Verifier().Verify(ctx, sandbox)
	if err == nil {
		t.Fatalf("Expected AST verification to fail on invalid code")
	}

	if !strings.Contains(err.Error(), "AST syntax or compilation failed") {
		t.Errorf("Expected error to mention syntax/compilation error, got: %v", err)
	}
}

func TestGoThinkSpace_Verify_MissingGoMod(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	space := setupGoThinkSpace(t, 5)
	sandbox := newMockSandbox()

	sandbox.WriteFile(ctx, "src/main.go", []byte("package main\nfunc main() {}\n"))

	err := space.Verifier().Verify(ctx, sandbox)
	if err == nil {
		t.Fatalf("Expected verification to fail due to missing go.mod")
	}

	if !strings.Contains(err.Error(), "no 'go.mod' file found") {
		t.Errorf("Expected missing go.mod error, got: %v", err)
	}
}

func TestGoThinkSpace_Verify_Success(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	space := setupGoThinkSpace(t, 15)
	sandbox := newMockSandbox()

	sandbox.WriteFile(ctx, "src/go.mod", []byte("module testmod\n\ngo 1.21\n"))
	sandbox.WriteFile(ctx, "src/main.go", []byte("package main\nfunc main() {}\n"))
	sandbox.WriteFile(ctx, "src/main_test.go", []byte("package main\nimport \"testing\"\nfunc TestMain(t *testing.T) {}\n"))

	err := space.Verifier().Verify(ctx, sandbox)
	if err != nil {
		t.Fatalf("Expected successful verification, got: %v", err)
	}
}

func TestGoThinkSpace_Verify_TimeoutLoop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	space := setupGoThinkSpace(t, 1)
	sandbox := newMockSandbox()
	sandbox.shouldDelay = true

	sandbox.WriteFile(ctx, "src/go.mod", []byte("module testmod\n\ngo 1.21\n"))
	sandbox.WriteFile(ctx, "src/main.go", []byte("package main\nfunc main() {}\n"))

	err := space.Verifier().Verify(ctx, sandbox)
	if err == nil {
		t.Fatalf("Expected verification to fail due to timeout")
	}

	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

// --- Mapbook Tests ---

func TestGolangMapbook_GenerateLayers(t *testing.T) {
	ctx := context.Background()
	workspaceRoot := t.TempDir()

	// Scaffold a fake Go workspace
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

	conceptual, ok := layers[assembler.LayerConceptual]
	if !ok || !strings.Contains(conceptual, "Standard Go project layout") {
		t.Errorf("missing or invalid conceptual layer")
	}

	structural, ok := layers[assembler.LayerStructural]
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
	workspaceRoot := t.TempDir() // Empty directory without a /src folder

	book := golang.NewGolangMapbook()
	layers, err := book.GenerateLayers(ctx, workspaceRoot)
	if err != nil {
		t.Fatalf("GenerateLayers should gracefully handle missing src directory, but got: %v", err)
	}

	structural, ok := layers[assembler.LayerStructural]
	if !ok {
		t.Fatalf("missing structural layer")
	}

	// Should just contain the header with no files listed
	if strings.Contains(structural, "- src/") {
		t.Errorf("expected empty structural tree, got: %s", structural)
	}
}

// --- Interface Contract Tests ---

func TestGoThinkSpace_InterfaceContracts(t *testing.T) {
	space := setupGoThinkSpace(t, 15)

	if space.Config().Name != "golang" {
		t.Errorf("Config() failed to return injected config")
	}

	if space.Verifier() == nil {
		t.Errorf("Verifier() returned nil")
	}

	if space.Mapbook() == nil {
		t.Errorf("Mapbook() returned nil")
	}

	tools := space.Tools()
	if len(tools) != 1 {
		t.Fatalf("Expected exactly 1 tool, got %d", len(tools))
	}

	funcDecl := tools[0].FunctionDeclarations[0]
	if funcDecl.Name != "propose_change" {
		t.Errorf("Expected tool name 'propose_change', got '%s'", funcDecl.Name)
	}
	if funcDecl.Description != "mock tool desc" {
		t.Errorf("Tool description was not correctly mapped from config")
	}

	props := funcDecl.Parameters.Properties
	if props["assigned_tags"].Description != "mock tags desc" {
		t.Errorf("Schema property mapping failed for assigned_tags")
	}
	if props["agent_tasks"].Items.Properties["target_files"].Description != "mock files desc" {
		t.Errorf("Schema property mapping failed for target_files inside agent_tasks")
	}
}
