package workbench

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Workbench manages file operations within a space's project artifact directory (<spaceID>/workbench).
type Workbench struct {
	rootDir string
}

func New(rootDir string) *Workbench {
	return &Workbench{rootDir: rootDir}
}

func (w *Workbench) Root() string {
	return w.rootDir
}

func (w *Workbench) resolveSafePath(targetPath string) (string, error) {
	if w.rootDir == "" {
		return "", fmt.Errorf("workbench root directory is not configured")
	}
	if targetPath == "" {
		return "", fmt.Errorf("target path cannot be empty")
	}

	cleaned := filepath.Clean(targetPath)
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("absolute path %q is not allowed in workbench", targetPath)
	}

	fullPath := filepath.Join(w.rootDir, cleaned)
	rel, err := filepath.Rel(w.rootDir, fullPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes workbench root", targetPath)
	}

	return fullPath, nil
}

func (w *Workbench) ResolvePath(targetPath string) string {
	fullPath, err := w.resolveSafePath(targetPath)
	if err != nil {
		return w.rootDir
	}
	return fullPath
}

func (w *Workbench) ReadFile(ctx context.Context, targetPath string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fullPath, err := w.resolveSafePath(targetPath)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(fullPath)
}

func (w *Workbench) WriteFile(ctx context.Context, targetPath string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	fullPath, err := w.resolveSafePath(targetPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, data, 0644)
}
