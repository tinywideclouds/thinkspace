package llm

import (
	"context"
	"fmt"
	"iter"

	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"google.golang.org/genai"
)

// ToolCall generically captures ANY function call requested by the LLM.
type ToolCall struct {
	Name string
	Args map[string]any
}

// Adapter bridges the ThinkSpace domain logic to the underlying GenAI client.
type Adapter struct {
	client ModelClient
}

func NewAdapter(client ModelClient) *Adapter {
	return &Adapter{client: client}
}

// GenerateStream dynamically accepts tools and the system prompt from the active ThinkSpace.
func (a *Adapter) GenerateStream(ctx context.Context, model string, systemPrompt string, tools []*genai.Tool, history []*genai.Content) iter.Seq2[*genai.GenerateContentResponse, error] {
	config := &genai.GenerateContentConfig{
		Tools:             tools,
		Temperature:       genai.Ptr(float32(0.2)),
		SystemInstruction: &genai.Content{Parts: []*genai.Part{{Text: systemPrompt}}},
	}
	return a.client.GenerateContentStream(ctx, model, history, config)
}

// InterceptToolCalls parses the raw LLM chunk to extract domain-agnostic tool requests.
func (a *Adapter) InterceptToolCalls(chunk *genai.GenerateContentResponse) []ToolCall {
	var calls []ToolCall
	if len(chunk.Candidates) == 0 || chunk.Candidates[0].Content == nil {
		return calls
	}

	for _, part := range chunk.Candidates[0].Content.Parts {
		if part.FunctionCall != nil {
			calls = append(calls, ToolCall{
				Name: part.FunctionCall.Name,
				Args: part.FunctionCall.Args,
			})
		}
	}
	return calls
}

// BuildHistory translates our domain events into the Google GenAI Content schema.
func (a *Adapter) BuildHistory(events []chat.Event) []*genai.Content {
	var history []*genai.Content

	for _, ev := range events {
		switch ev.Type {
		case chat.EventPrompt:
			history = append(history, &genai.Content{
				Role:  "user",
				Parts: []*genai.Part{{Text: ev.Content}},
			})
		case chat.EventModel:
			history = append(history, &genai.Content{
				Role:  "model",
				Parts: []*genai.Part{{Text: ev.Content}},
			})
		case chat.EventCandidate:
			// Inject the proposal knowledge as an action the model took
			text := fmt.Sprintf("[Action Taken: Proposed files (%s) under proposal UID: %s]", ev.Metadata["files"], ev.Metadata["proposal_uid"])
			history = append(history, &genai.Content{
				Role:  "model",
				Parts: []*genai.Part{{Text: text}},
			})
		case chat.EventResolution:
			// Inject the outcome as feedback from the user/system
			text := fmt.Sprintf("[System Feedback: Proposal %s was %s. Reason: %s]", ev.Metadata["proposal_uid"], ev.Metadata["status"], ev.Metadata["reason"])
			history = append(history, &genai.Content{
				Role:  "user",
				Parts: []*genai.Part{{Text: text}},
			})
		}
	}

	return history
}

func safeString(val any) string {
	if val == nil {
		return ""
	}
	return val.(string)
}
