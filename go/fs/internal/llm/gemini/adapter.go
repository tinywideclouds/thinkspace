package gemini

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"strings"

	"google.golang.org/genai"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	geminitools "github.com/tinywideclouds.com/thinkspace/internal/session/tools/gemini"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
)

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
	registry *tools.Registry,
	config spaces.ThinkStateConfig,
) iter.Seq2[llm.StreamChunk, error] {

	var genaiTools []*genai.Tool
	if registry != nil {
		var transition *tools.TransitionThinkStateTool
		var propose *tools.ProposeChangeTool
		var query *tools.QueryLensTool
		var digest *tools.CreateDigestTool

		if t, ok := registry.Get(string(tools.TransitionThinkState)); ok {
			transition, _ = t.(*tools.TransitionThinkStateTool)
		}
		if p, ok := registry.Get(string(tools.ProposeChange)); ok {
			propose, _ = p.(*tools.ProposeChangeTool)
		}
		if q, ok := registry.Get(string(tools.QueryLens)); ok {
			query, _ = q.(*tools.QueryLensTool)
		}
		if d, ok := registry.Get(string(tools.CreateDigest)); ok {
			digest, _ = d.(*tools.CreateDigestTool)
		}

		genaiTools = geminitools.CompileCoreTools(transition, propose, query, digest, config)
	}

	sysPrompt := a.compileSystemInstruction(bundle)
	genaiConfig := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{Parts: []*genai.Part{{Text: sysPrompt}}},
		Tools:             genaiTools,
		Temperature:       genai.Ptr(float32(0.2)),
	}

	history := a.compileHistory(bundle)
	rawStream := a.client.Models.GenerateContentStream(ctx, model, history, genaiConfig)

	return func(yield func(llm.StreamChunk, error) bool) {
		chunkIndex := 0
		for chunk, err := range rawStream {
			if err != nil {
				slog.ErrorContext(ctx, "gemini stream read error", "model", model, "error", err)
				yield(llm.StreamChunk{}, err)
				return
			}
			chunkIndex++

			parsedChunk := llm.StreamChunk{}
			if len(chunk.Candidates) > 0 {
				candidate := chunk.Candidates[0]
				if candidate.FinishReason != "" && candidate.FinishReason != genai.FinishReasonStop {
					slog.WarnContext(ctx, "gemini non-standard finish reason",
						"model", model,
						"finish_reason", candidate.FinishReason,
					)
				}

				if candidate.Content != nil {
					for _, part := range candidate.Content.Parts {
						if part.Text != "" {
							parsedChunk.Text += part.Text
						}
						if part.FunctionCall != nil {
							var argKeys []string
							for k := range part.FunctionCall.Args {
								argKeys = append(argKeys, k)
							}
							slog.InfoContext(ctx, "gemini stream yielded function call",
								"model", model,
								"tool_name", part.FunctionCall.Name,
								"args", argKeys,
							)
							parsedChunk.ToolCalls = append(parsedChunk.ToolCalls, llm.ToolCall{
								Name: part.FunctionCall.Name,
								Args: part.FunctionCall.Args,
							})
						}
					}
				}
			}

			if chunk.UsageMetadata != nil {
				parsedChunk.Usage = &llm.UsageMetadata{
					PromptTokens:    int(chunk.UsageMetadata.PromptTokenCount),
					CandidateTokens: int(chunk.UsageMetadata.CandidatesTokenCount),
					TotalTokens:     int(chunk.UsageMetadata.TotalTokenCount),
				}
			}

			if !yield(parsedChunk, nil) {
				return
			}
		}

		if chunkIndex == 0 {
			slog.WarnContext(ctx, "gemini stream returned 0 chunks from API", "model", model)
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
