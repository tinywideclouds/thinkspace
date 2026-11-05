package llm_test

import (
	"context"
	"iter"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
)

type MockModelClient struct {
	Responses []llm.StreamChunk
	Err       error
}

func (m *MockModelClient) GenerateContentStream(
	ctx context.Context,
	model string,
	bundle cognition.ThinkBundle,
	registry *tools.Registry,
	config spaces.ThinkStateConfig,
) iter.Seq2[llm.StreamChunk, error] {
	return func(yield func(llm.StreamChunk, error) bool) {
		if m.Err != nil {
			yield(llm.StreamChunk{}, m.Err)
			return
		}
		for _, resp := range m.Responses {
			if !yield(resp, nil) {
				return
			}
		}
	}
}
