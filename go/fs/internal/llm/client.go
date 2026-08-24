package llm

import (
	"context"
	"iter"

	"google.golang.org/genai"
)

// ModelClient defines the interface for streaming LLM generation, allowing for mock implementations.
type ModelClient interface {
	GenerateContentStream(ctx context.Context, model string, history []*genai.Content, config *genai.GenerateContentConfig) iter.Seq2[*genai.GenerateContentResponse, error]
}

// GenAIClient is the physical implementation that wraps the official Google GenAI SDK.
type GenAIClient struct {
	client *genai.Client
}

func NewGenAIClient(client *genai.Client) *GenAIClient {
	return &GenAIClient{
		client: client,
	}
}

func (c *GenAIClient) GenerateContentStream(ctx context.Context, model string, history []*genai.Content, config *genai.GenerateContentConfig) iter.Seq2[*genai.GenerateContentResponse, error] {
	return c.client.Models.GenerateContentStream(ctx, model, history, config)
}
