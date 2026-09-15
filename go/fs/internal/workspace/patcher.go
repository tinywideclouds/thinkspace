package workspace

import "context"

// Patcher defines how an LLM's raw output is interpreted and applied to physical files.
type Patcher interface {
	Apply(ctx context.Context, sandbox CandidateSandbox, llmOutput string) error

	// SystemInstructions returns the specific formatting rules the LLM must follow
	// to use this patcher (e.g., JSON schemas, Markdown block formats).
	SystemInstructions() string
}
