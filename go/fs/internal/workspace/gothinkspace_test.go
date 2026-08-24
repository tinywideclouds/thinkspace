package workspace_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

func setupGoThinkSpace(t *testing.T, verifyTimeout int) *workspace.GoThinkSpace {
	config := workspace.ThinkSpaceConfig{
		Name:                 "golang",
		VerifyTimeoutSeconds: verifyTimeout,
	}
	config.ApplyDefaults()
	return workspace.NewGoThinkSpace(config)
}

func TestGoThinkSpace_Verify_ASTError(t *testing.T) {
	space := setupGoThinkSpace(t, 5)
	dir := t.TempDir()

	invalidGoCode := "package main\nfunc main() {\n"
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(invalidGoCode), 0644)

	err := space.Verify(context.Background(), dir)
	if err == nil {
		t.Fatalf("Expected AST verification to fail on invalid code")
	}

	if !strings.Contains(err.Error(), "Syntax error") {
		t.Errorf("Expected error to mention syntax error, got: %v", err)
	}
}

func TestGoThinkSpace_Verify_MissingGoMod(t *testing.T) {
	space := setupGoThinkSpace(t, 5)
	dir := t.TempDir()

	validGoCode := "package main\nfunc main() {}\n"
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(validGoCode), 0644)

	err := space.Verify(context.Background(), dir)
	if err == nil {
		t.Fatalf("Expected verification to fail due to missing go.mod")
	}

	if !strings.Contains(err.Error(), "no 'go.mod' file found") {
		t.Errorf("Expected missing go.mod error, got: %v", err)
	}
}

func TestGoThinkSpace_Verify_Success(t *testing.T) {
	space := setupGoThinkSpace(t, 15)
	dir := t.TempDir()

	goMod := "module testmod\n\ngo 1.21\n"
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644)

	validGoCode := "package main\nfunc main() {}\n"
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(validGoCode), 0644)

	testCode := "package main\nimport \"testing\"\nfunc TestMain(t *testing.T) {}\n"
	os.WriteFile(filepath.Join(dir, "main_test.go"), []byte(testCode), 0644)

	err := space.Verify(context.Background(), dir)
	if err != nil {
		t.Fatalf("Expected successful verification, got: %v", err)
	}
}

func TestGoThinkSpace_Verify_TimeoutLoop(t *testing.T) {
	// 1 second timeout to quickly catch the infinite loop
	space := setupGoThinkSpace(t, 1)
	dir := t.TempDir()

	goMod := "module testmod\n\ngo 1.21\n"
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644)

	validGoCode := "package main\nfunc main() {}\n"
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(validGoCode), 0644)

	// A test that sleeps for 5 seconds, deliberately violating the 1-second verify context
	testCode := "package main\nimport (\"testing\"; \"time\")\nfunc TestInfiniteLoop(t *testing.T) { time.Sleep(5 * time.Second) }\n"
	os.WriteFile(filepath.Join(dir, "main_test.go"), []byte(testCode), 0644)

	err := space.Verify(context.Background(), dir)
	if err == nil {
		t.Fatalf("Expected verification to fail due to timeout")
	}

	if !strings.Contains(err.Error(), "timed out (possible infinite loop)") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}
