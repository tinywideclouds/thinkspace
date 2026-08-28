package golang_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace/spaces/golang"
)

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
		// We expect the files map to simulate the src/ directory contract
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
	config := workspace.ThinkSpaceConfig{
		Name:                 "golang",
		VerifyTimeoutSeconds: verifyTimeout,
	}
	config.ApplyDefaults()
	return golang.NewGoThinkSpace(config)
}

func TestGoThinkSpace_Verify_ASTError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	space := setupGoThinkSpace(t, 5)
	sandbox := newMockSandbox()

	// Update paths to include src/
	sandbox.WriteFile(ctx, "src/go.mod", []byte("module test"))
	sandbox.WriteFile(ctx, "src/main.go", []byte("package main\nfunc invalid() {\n"))

	err := space.Verify(ctx, sandbox)
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

	// Agent writes code but forgets go.mod
	sandbox.WriteFile(ctx, "src/main.go", []byte("package main\nfunc main() {}\n"))

	err := space.Verify(ctx, sandbox)
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

	// Everything properly inside src/
	sandbox.WriteFile(ctx, "src/go.mod", []byte("module testmod\n\ngo 1.21\n"))
	sandbox.WriteFile(ctx, "src/main.go", []byte("package main\nfunc main() {}\n"))
	sandbox.WriteFile(ctx, "src/main_test.go", []byte("package main\nimport \"testing\"\nfunc TestMain(t *testing.T) {}\n"))

	err := space.Verify(ctx, sandbox)
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

	err := space.Verify(ctx, sandbox)
	if err == nil {
		t.Fatalf("Expected verification to fail due to timeout")
	}

	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}
