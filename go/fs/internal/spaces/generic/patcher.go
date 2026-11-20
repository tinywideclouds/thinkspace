package generic

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type SearchReplacePatcher struct {
	instructions string
}

func NewSearchReplacePatcher(instructions string) *SearchReplacePatcher {
	return &SearchReplacePatcher{
		instructions: instructions,
	}
}

func (p *SearchReplacePatcher) SystemInstructions() string {
	return p.instructions
}

func (p *SearchReplacePatcher) Apply(ctx context.Context, fs workspace.FileWorkspace, llmOutput string) error {
	// Match blocks formatted as:
	// ### src/file.go
	// <<<< SEARCH
	// ...
	// ====
	// ...
	// >>>> REPLACE
	blockRegex := regexp.MustCompile(`(?s)###\s*(.+?)\s*<<<< SEARCH\s*(.*?)\s*====\s*(.*?)\s*>>>> REPLACE`)
	matches := blockRegex.FindAllStringSubmatch(llmOutput, -1)

	if len(matches) == 0 {
		return fmt.Errorf("no valid SEARCH/REPLACE blocks found in output")
	}

	fileModifications := make(map[string]string)
	var fileOrder []string

	for _, match := range matches {
		if len(match) != 4 {
			continue
		}

		rawPath := strings.TrimSpace(match[1])
		if rawPath == "" {
			return fmt.Errorf("patch block is missing target file path")
		}

		filePath := filepath.ToSlash(filepath.Clean(rawPath))
		if filepath.IsAbs(filePath) || filePath == "." || filePath == ".." || strings.HasPrefix(filePath, "../") {
			return fmt.Errorf("invalid or escaping target file path: %q", rawPath)
		}

		searchBlock := match[2]
		replaceBlock := match[3]

		if strings.TrimSpace(searchBlock) == "" {
			return fmt.Errorf("empty SEARCH block is not allowed for %s", filePath)
		}

		content, exists := fileModifications[filePath]
		if !exists {
			rawBytes, err := fs.ReadFile(ctx, filePath)
			if err != nil {
				return fmt.Errorf("cannot patch missing or unreadable file %s: %w", filePath, err)
			}
			content = string(rawBytes)
			fileOrder = append(fileOrder, filePath)
		}

		if !strings.Contains(content, searchBlock) {
			return fmt.Errorf("search block not found in %s:\n%s", filePath, searchBlock)
		}

		fileModifications[filePath] = strings.Replace(content, searchBlock, replaceBlock, 1)
	}

	for _, filePath := range fileOrder {
		if err := fs.WriteFile(ctx, filePath, []byte(fileModifications[filePath])); err != nil {
			return fmt.Errorf("writing patched file %s: %w", filePath, err)
		}
	}

	return nil
}
