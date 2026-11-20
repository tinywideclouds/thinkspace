package workbench_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/workbench"
)

func TestWorkbench_ReadWriteAndContainment(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	wb := workbench.New(root)

	if wb.Root() != root {
		t.Errorf("expected Root() %q, got %q", root, wb.Root())
	}

	if err := wb.WriteFile(ctx, "src/main.go", []byte("package main\n")); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	expectedResolved := filepath.Join(root, "src", "main.go")
	if got := wb.ResolvePath("src/main.go"); got != expectedResolved {
		t.Errorf("expected ResolvePath %q, got %q", expectedResolved, got)
	}

	data, err := wb.ReadFile(ctx, "src/main.go")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(data) != "package main\n" {
		t.Errorf("expected 'package main\\n', got %q", string(data))
	}

	if err := wb.WriteFile(ctx, "../outside.txt", []byte("evil")); err == nil {
		t.Errorf("expected WriteFile to reject path traversal")
	}
	if _, err := wb.ReadFile(ctx, "../outside.txt"); err == nil {
		t.Errorf("expected ReadFile to reject path traversal")
	}
}
