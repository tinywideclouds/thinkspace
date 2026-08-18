package llm

import (
	"context"
	"fmt"
	"google.golang.org/genai"
	"iter"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type ToolCall struct {
	FilePath   string
	Patch      string
	NewContent string
	Reasoning  string
}

type Manager struct {
	client *genai.Client
}

func NewManager(client *genai.Client) *Manager {
	return &Manager{client: client}
}

func (m *Manager) GenerateStream(ctx context.Context, model string, history []*genai.Content) iter.Seq2[*genai.GenerateContentResponse, error] {
	config := &genai.GenerateContentConfig{
		Tools:       GetWorkspaceTools(),
		Temperature: genai.Ptr(float32(0.2)),
	}
	return m.client.Models.GenerateContentStream(ctx, model, history, config)
}

func (m *Manager) InterceptToolCalls(chunk *genai.GenerateContentResponse) []ToolCall {
	var calls []ToolCall
	if len(chunk.Candidates) == 0 || chunk.Candidates[0].Content == nil {
		return calls
	}

	for _, part := range chunk.Candidates[0].Content.Parts {
		if part.FunctionCall != nil && part.FunctionCall.Name == "propose_change" {
			args := part.FunctionCall.Args
			calls = append(calls, ToolCall{
				FilePath:   args["file_path"].(string),
				Patch:      safeString(args["patch"]),
				NewContent: safeString(args["new_content"]),
				Reasoning:  safeString(args["reasoning"]),
			})
		}
	}
	return calls
}

// BuildHistory translates our domain events into the Google GenAI Content schema.
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
