package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type WorkbenchReader struct {
	projectDir string
	chatID     string
}

func (r *WorkbenchReader) ReadFile(ctx context.Context, targetPath string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cleaned := filepath.Clean(targetPath)
	if targetPath == "" || filepath.IsAbs(cleaned) || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("invalid target file path %q", targetPath)
	}

	slashPath := filepath.ToSlash(cleaned)
	if r.chatID != "" {
		chatPrefix := fmt.Sprintf("chats/%s/", r.chatID)
		if strings.HasPrefix(slashPath, chatPrefix) {
			jailedRel := strings.TrimPrefix(slashPath, chatPrefix)
			jailedRoot := filepath.Join(r.projectDir, "chats", r.chatID)
			fullPath := filepath.Join(jailedRoot, filepath.FromSlash(jailedRel))
			if rel, err := filepath.Rel(jailedRoot, fullPath); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return os.ReadFile(fullPath)
			}
			return nil, fmt.Errorf("path %q escapes chat workspace", targetPath)
		}
	}

	fullPath := filepath.Join(r.projectDir, filepath.FromSlash(slashPath))
	rel, err := filepath.Rel(r.projectDir, fullPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("path %q escapes project workspace", targetPath)
	}

	data, readErr := os.ReadFile(fullPath)
	if errors.Is(readErr, os.ErrNotExist) && r.chatID != "" {
		jailedRoot := filepath.Join(r.projectDir, "chats", r.chatID)
		jailedPath := filepath.Join(jailedRoot, filepath.FromSlash(slashPath))
		if jRel, jErr := filepath.Rel(jailedRoot, jailedPath); jErr == nil && jRel != ".." && !strings.HasPrefix(jRel, ".."+string(filepath.Separator)) {
			if jailedData, jailedErr := os.ReadFile(jailedPath); jailedErr == nil {
				return jailedData, nil
			}
		}
	}
	return data, readErr
}
