package llm

import (
	"context"
	"iter"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
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

type ModelClient interface {
	GenerateContentStream(
		ctx context.Context,
		model string,
		bundle cognition.ThinkBundle,
		registry *tools.Registry,
		config spaces.ThinkStateConfig,
	) iter.Seq2[StreamChunk, error]
}
