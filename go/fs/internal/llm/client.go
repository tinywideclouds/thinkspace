package llm

import (
	"context"
	"iter"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
)

type StreamChunk struct {
	Text      string
	ToolCalls []ToolCall
}

type ToolCall struct {
	Name string
	Args map[string]any
}

type ToolDef struct {
	Name        string
	Description string
	Parameters  map[string]any
	Required    []string
}

// ModelClient defines the domain-agnostic interface for streaming LLM generation.
type ModelClient interface {
	GenerateContentStream(
		ctx context.Context,
		model string,
		bundle cognition.ThinkBundle,
		tools []ToolDef,
	) iter.Seq2[StreamChunk, error]
}
