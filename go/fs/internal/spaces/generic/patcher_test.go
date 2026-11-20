package generic_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces/generic"
)

type genericMockWorkspace struct {
	files        map[string][]byte
	executedCmds []string
	failCmds     map[string]error
}

func newGenericMockWorkspace() *genericMockWorkspace {
	return &genericMockWorkspace{
		files:    make(map[string][]byte),
		failCmds: make(map[string]error),
	}
}

func (m *genericMockWorkspace) ResolvePath(path string) string { return path }
func (m *genericMockWorkspace) WriteFile(ctx context.Context, path string, data []byte) error {
	m.files[path] = data
	return nil
}
func (m *genericMockWorkspace) ReadFile(ctx context.Context, path string) ([]byte, error) {
	data, ok := m.files[path]
	if !ok {
		return nil, errors.New("not found")
	}
	return data, nil
}
func (m *genericMockWorkspace) ExecuteCommand(ctx context.Context, command string, args ...string) (string, error) {
	full := strings.TrimSpace(command + " " + strings.Join(args, " "))
	m.executedCmds = append(m.executedCmds, full)
	if err, ok := m.failCmds[full]; ok {
		return "cmd failed", err
	}
	return "ok", nil
}

func TestSearchReplacePatcher_ApplyAndValidation(t *testing.T) {
	ctx := context.Background()
	ws := newGenericMockWorkspace()
	ws.files["docs/spec.md"] = []byte("# Title\nOld paragraph.\nFooter.")

	patcher := generic.NewSearchReplacePatcher("Use search/replace")
	if patcher.SystemInstructions() != "Use search/replace" {
		t.Errorf("unexpected SystemInstructions: %q", patcher.SystemInstructions())
	}

	patchInput := `
### docs/spec.md
<<<< SEARCH
Old paragraph.
====
New updated paragraph.
>>>> REPLACE
`
	if err := patcher.Apply(ctx, ws, patchInput); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	got := string(ws.files["docs/spec.md"])
	if !strings.Contains(got, "New updated paragraph.") || strings.Contains(got, "Old paragraph.") {
		t.Errorf("unexpected patched content:\n%s", got)
	}

	// Reject path traversal
	traversalPatch := "### ../outside.md\n<<<< SEARCH\na\n====\nb\n>>>> REPLACE"
	if err := patcher.Apply(ctx, ws, traversalPatch); err == nil {
		t.Errorf("expected path traversal patch to fail")
	}

	// Reject empty SEARCH block
	emptySearchPatch := "### docs/spec.md\n<<<< SEARCH\n   \n====\ninjected\n>>>> REPLACE"
	if err := patcher.Apply(ctx, ws, emptySearchPatch); err == nil {
		t.Errorf("expected empty SEARCH block to fail")
	}
}

func TestCIVerifier_PreVerifyNonStrictAndStrictVerify(t *testing.T) {
	ctx := context.Background()
	ws := newGenericMockWorkspace()
	ws.failCmds["fmt-tool"] = errors.New("formatter warning")

	verifier := generic.NewCIVerifier(spaces.ToolchainConfig{
		PreVerify: []string{"fmt-tool"},
		Verify:    []string{"check-tool"},
	}, 5*time.Second)

	// Non-strict PreVerify failure should NOT block strict Verify from passing
	if err := verifier.Verify(ctx, ws); err != nil {
		t.Fatalf("expected non-strict PreVerify failure to be tolerated when Verify passes, got: %v", err)
	}
	if len(ws.executedCmds) != 2 {
		t.Errorf("expected both fmt-tool and check-tool to run, got %v", ws.executedCmds)
	}
}
