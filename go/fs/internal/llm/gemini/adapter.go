package gemini

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"sort"
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
	if a == nil || a.client == nil {
		return func(yield func(llm.StreamChunk, error) bool) {
			yield(llm.StreamChunk{}, fmt.Errorf("gemini client is not initialized"))
		}
	}

	var genaiTools []*genai.Tool
	if registry != nil {
		var transition *tools.TransitionThinkStateTool
		var propose *tools.ProposeChangeTool
		var query *tools.QueryLensTool
		var digest *tools.CreateDigestTool

		if t, ok := registry.Get(string(tools.TransitionThinkState)); ok {
			var typeOK bool
			transition, typeOK = t.(*tools.TransitionThinkStateTool)
			if !typeOK {
				slog.WarnContext(ctx, "registered tool did not match expected type", "tool", tools.TransitionThinkState, "actual_type", fmt.Sprintf("%T", t))
			}
		}
		if p, ok := registry.Get(string(tools.ProposeChange)); ok {
			var typeOK bool
			propose, typeOK = p.(*tools.ProposeChangeTool)
			if !typeOK {
				slog.WarnContext(ctx, "registered tool did not match expected type", "tool", tools.ProposeChange, "actual_type", fmt.Sprintf("%T", p))
			}
		}
		if q, ok := registry.Get(string(tools.QueryLens)); ok {
			var typeOK bool
			query, typeOK = q.(*tools.QueryLensTool)
			if !typeOK {
				slog.WarnContext(ctx, "registered tool did not match expected type", "tool", tools.QueryLens, "actual_type", fmt.Sprintf("%T", q))
			}
		}
		if d, ok := registry.Get(string(tools.CreateDigest)); ok {
			var typeOK bool
			digest, typeOK = d.(*tools.CreateDigestTool)
			if !typeOK {
				slog.WarnContext(ctx, "registered tool did not match expected type", "tool", tools.CreateDigest, "actual_type", fmt.Sprintf("%T", d))
			}
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
							argKeys := make([]string, 0, len(part.FunctionCall.Args))
							for k := range part.FunctionCall.Args {
								argKeys = append(argKeys, k)
							}
							sort.Strings(argKeys)
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
		mapNames := make([]string, 0, len(bundle.Mapbook.Maps))
		for mapName := range bundle.Mapbook.Maps {
			mapNames = append(mapNames, mapName)
		}
		sort.Strings(mapNames)
		for _, mapName := range mapNames {
			b.WriteString(fmt.Sprintf("\n--- %s ---\n%s\n", mapName, bundle.Mapbook.Maps[mapName]))
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
	catalystRole := bundle.Trajectory.Catalyst.Role
	if catalystRole == "" {
		catalystRole = "user"
	}
	history = append(history, &genai.Content{
		Role:  catalystRole,
		Parts: []*genai.Part{{Text: catalyst.String()}},
	})
	return history
}
