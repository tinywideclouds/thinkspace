package gemini

import (
	"context"
	"fmt"
	"iter"
	"strings"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"google.golang.org/genai"
)

// Adapter implements llm.ModelClient for Google's GenAI SDK.
type Adapter struct {
	client *genai.Client
}

func NewAdapter(client *genai.Client) *Adapter {
	return &Adapter{client: client}
}

func (a *Adapter) GenerateContentStream(
	ctx context.Context,
	model string,
	bundle cognition.ThinkBundle,
	tools []llm.ToolDef,
) iter.Seq2[llm.StreamChunk, error] {

	sysPrompt := a.compileSystemInstruction(bundle)
	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{Parts: []*genai.Part{{Text: sysPrompt}}},
		Tools:             a.compileTools(tools),
		Temperature:       genai.Ptr(float32(0.2)),
	}

	history := a.compileHistory(bundle)
	rawStream := a.client.Models.GenerateContentStream(ctx, model, history, config)

	return func(yield func(llm.StreamChunk, error) bool) {
		for chunk, err := range rawStream {
			if err != nil {
				yield(llm.StreamChunk{}, err)
				return
			}

			parsedChunk := llm.StreamChunk{}
			if len(chunk.Candidates) > 0 && chunk.Candidates[0].Content != nil {
				for _, part := range chunk.Candidates[0].Content.Parts {
					if part.Text != "" {
						parsedChunk.Text += part.Text
					}
					if part.FunctionCall != nil {
						parsedChunk.ToolCalls = append(parsedChunk.ToolCalls, llm.ToolCall{
							Name: part.FunctionCall.Name,
							Args: part.FunctionCall.Args,
						})
					}
				}
			}
			if !yield(parsedChunk, nil) {
				return
			}
		}
	}
}

func (a *Adapter) compileSystemInstruction(bundle cognition.ThinkBundle) string {
	var b strings.Builder

	b.WriteString(bundle.Rules.Persona)
	if bundle.Rules.FormattingConstraints != "" {
		b.WriteString("\n\n" + bundle.Rules.FormattingConstraints)
	}

	if len(bundle.Mapbook.Maps) > 0 {
		b.WriteString("\n\n### THE MAPBOOK\n")
		for mapName, mapContent := range bundle.Mapbook.Maps {
			b.WriteString(fmt.Sprintf("\n--- %s ---\n%s\n", mapName, mapContent))
		}
	}

	return b.String()
}

func (a *Adapter) compileHistory(bundle cognition.ThinkBundle) []*genai.Content {
	var history []*genai.Content

	for _, memory := range bundle.Trajectory.LongTermMemory {
		history = append(history, &genai.Content{
			Role:  "user",
			Parts: []*genai.Part{{Text: "[Background Memory]: " + memory}},
		})
		history = append(history, &genai.Content{
			Role:  "model",
			Parts: []*genai.Part{{Text: "Understood. I will keep this in mind."}},
		})
	}

	for _, event := range bundle.Trajectory.WorkingMemory {
		history = append(history, &genai.Content{
			Role:  event.Role,
			Parts: []*genai.Part{{Text: event.Content}},
		})
	}

	var catalyst strings.Builder
	if len(bundle.Workbench) > 0 {
		catalyst.WriteString("### Materialized Workbench\n")
		for _, item := range bundle.Workbench {
			catalyst.WriteString(fmt.Sprintf("\n--- %s ---\n```\n%s\n```\n", item.Path, item.Content))
		}
		catalyst.WriteString("\n")
	}

	catalyst.WriteString(bundle.Trajectory.Catalyst.Content)

	history = append(history, &genai.Content{
		Role:  bundle.Trajectory.Catalyst.Role,
		Parts: []*genai.Part{{Text: catalyst.String()}},
	})

	return history
}

func (a *Adapter) compileTools(tools []llm.ToolDef) []*genai.Tool {
	if len(tools) == 0 {
		return nil
	}
	var declarations []*genai.FunctionDeclaration
	for _, t := range tools {
		decl := &genai.FunctionDeclaration{
			Name:        t.Name,
			Description: t.Description,
		}
		declarations = append(declarations, decl)
	}
	return []*genai.Tool{{FunctionDeclarations: declarations}}
}
