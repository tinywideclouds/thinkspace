package assembler

import (
	"context"
	"fmt"
	"strings"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// Workbench assembles the Just-In-Time file context requested by the Manager.
type Workbench struct{}

func NewWorkbench() *Workbench {
	return &Workbench{}
}

// Build reads requested files from the agent's sandbox.
// It fails fast and returns an error if the Manager hallucinated a file path.
func (w *Workbench) Build(ctx context.Context, sandbox workspace.CandidateSandbox, targetFiles []string) (string, error) {
	if len(targetFiles) == 0 {
		return "", nil
	}

	var workbenchContext strings.Builder
	workbenchContext.WriteString("\n\n### Workbench Files (Current State)\n")

	for _, filePath := range targetFiles {
		content, err := sandbox.ReadFile(ctx, filePath)
		if err != nil || len(content) == 0 {
			return "", fmt.Errorf("fail-fast: requested target file '%s' does not exist or is empty", filePath)
		}
		workbenchContext.WriteString(fmt.Sprintf("\n--- %s ---\n```\n%s\n```\n", filePath, string(content)))
	}

	return workbenchContext.String(), nil
}
