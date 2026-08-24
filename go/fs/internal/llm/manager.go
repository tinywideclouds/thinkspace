package llm

import (
	"context"
	"fmt"
	"iter"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"google.golang.org/genai"
)

// ToolCall now generically captures ANY function call requested by the LLM.
type ToolCall struct {
	Name string
	Args map[string]any
}

type Manager struct {
	client ModelClient
}

func NewManager(client ModelClient) *Manager {
	return &Manager{client: client}
}

// GenerateStream dynamically accepts tools and the system prompt from the active ThinkSpace.
func (m *Manager) GenerateStream(ctx context.Context, model string, systemPrompt string, tools []*genai.Tool, history []*genai.Content) iter.Seq2[*genai.GenerateContentResponse, error] {
	config := &genai.GenerateContentConfig{
		Tools:             tools,
		Temperature:       genai.Ptr(float32(0.2)),
		SystemInstruction: &genai.Content{Parts: []*genai.Part{{Text: systemPrompt}}},
	}
	return m.client.GenerateContentStream(ctx, model, history, config)
}

// InterceptToolCalls is now completely domain-agnostic.
func (m *Manager) InterceptToolCalls(chunk *genai.GenerateContentResponse) []ToolCall {
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
func (m *Manager) BuildHistory(events []workspace.Event) []*genai.Content {
	var history []*genai.Content

	for _, ev := range events {
		switch ev.Type {
		case workspace.EventPrompt:
			history = append(history, &genai.Content{
				Role:  "user",
				Parts: []*genai.Part{{Text: ev.Content}},
			})
		case workspace.EventModel:
			history = append(history, &genai.Content{
				Role:  "model",
				Parts: []*genai.Part{{Text: ev.Content}},
			})
		case workspace.EventCandidate:
			// Inject the proposal knowledge as an action the model took
			text := fmt.Sprintf("[Action Taken: Proposed files (%s) under proposal UID: %s]", ev.Metadata["files"], ev.Metadata["proposal_uid"])
			history = append(history, &genai.Content{
				Role:  "model",
				Parts: []*genai.Part{{Text: text}},
			})
		case workspace.EventResolution:
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
