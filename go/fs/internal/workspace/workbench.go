package workspace

import (
	"context"
	"fmt"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
)

// Workbench assembles the Just-In-Time file context requested by the Manager.
type Workbench struct{}

func NewWorkbench() *Workbench {
	return &Workbench{}
}

// Build reads requested files from the agent's sandbox.
// It fails fast and returns an error if the Manager hallucinated a file path.
func (w *Workbench) Build(ctx context.Context, sandbox CandidateSandbox, targetFiles []string) ([]cognition.WorkbenchItem, error) {
	if len(targetFiles) == 0 {
		return nil, nil
	}

	var items []cognition.WorkbenchItem

	for _, filePath := range targetFiles {
		content, err := sandbox.ReadFile(ctx, filePath)
		if err != nil || len(content) == 0 {
			return nil, fmt.Errorf("fail-fast: requested target file '%s' does not exist or is empty", filePath)
		}
		items = append(items, cognition.WorkbenchItem{
			Path:       filePath,
			Content:    string(content),
			TokenCount: 0, // Placeholder for future token counting integration
		})
	}

	return items, nil
}
