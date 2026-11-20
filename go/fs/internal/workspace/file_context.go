package workspace

import (
	"context"
	"fmt"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
)

// ReadFileContext reads requested target files from a FileReader and estimates token counts.
// It fails fast and returns an error if any requested file path does not exist or cannot be read.
func ReadFileContext(ctx context.Context, reader FileReader, targetFiles []string) ([]cognition.WorkbenchItem, error) {
	if len(targetFiles) == 0 {
		return nil, nil
	}

	items := make([]cognition.WorkbenchItem, 0, len(targetFiles))
	for _, filePath := range targetFiles {
		content, err := reader.ReadFile(ctx, filePath)
		if err != nil {
			return nil, fmt.Errorf("fail-fast: requested target file '%s' does not exist or cannot be read: %w", filePath, err)
		}
		tokenCount := len(content) / 4
		if len(content) > 0 && tokenCount == 0 {
			tokenCount = 1
		}
		items = append(items, cognition.WorkbenchItem{
			Path:       filePath,
			Content:    string(content),
			TokenCount: tokenCount,
		})
	}

	return items, nil
}
