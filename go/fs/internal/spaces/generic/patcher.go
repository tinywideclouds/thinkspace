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

func (p *SearchReplacePatcher) Apply(ctx context.Context, sandbox workspace.CandidateSandbox, llmOutput string) error {
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

	for _, match := range matches {
		if len(match) != 4 {
			continue
		}

		filePath := filepath.ToSlash(filepath.Clean(strings.TrimSpace(match[1])))
		searchBlock := match[2]
		replaceBlock := match[3]

		// Load from sandbox if not already loaded in this batch
		content, exists := fileModifications[filePath]
		if !exists {
			bytes, err := sandbox.ReadFile(ctx, filePath)
			if err != nil {
				return fmt.Errorf("cannot patch missing file %s", filePath)
			}
			content = string(bytes)
		}

		if !strings.Contains(content, searchBlock) {
			return fmt.Errorf("search block not found in %s:\n%s", filePath, searchBlock)
		}

		// Apply the string replacement
		fileModifications[filePath] = strings.Replace(content, searchBlock, replaceBlock, 1)
	}

	// Flush modifications to the physical sandbox
	for filePath, newContent := range fileModifications {
		if err := sandbox.WriteFile(ctx, filePath, []byte(newContent)); err != nil {
			return fmt.Errorf("writing patched file %s: %w", filePath, err)
		}
	}

	return nil
}
