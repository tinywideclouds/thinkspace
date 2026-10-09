package llm

import (
	"context"
	"iter"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
)

type UsageMetadata struct {
	PromptTokens    int
	CandidateTokens int
	TotalTokens     int
}

type StreamChunk struct {
	Text      string
	ToolCalls []ToolCall
	Usage     *UsageMetadata
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
