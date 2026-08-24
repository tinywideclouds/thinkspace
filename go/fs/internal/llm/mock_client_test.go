package llm_test

import (
	"context"
	"iter"

	"google.golang.org/genai"
)

// MockModelClient is a deterministic implementation of the ModelClient interface for testing.
type MockModelClient struct {
	Responses []*genai.GenerateContentResponse
	Err       error
}

func (m *MockModelClient) GenerateContentStream(ctx context.Context, model string, history []*genai.Content, config *genai.GenerateContentConfig) iter.Seq2[*genai.GenerateContentResponse, error] {
	return func(yield func(*genai.GenerateContentResponse, error) bool) {
		if m.Err != nil {
			yield(nil, m.Err)
			return
		}
		for _, resp := range m.Responses {
			if !yield(resp, nil) {
				return
			}
		}
	}
}
