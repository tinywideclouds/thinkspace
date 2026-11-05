package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type TraceEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Prompt    string    `json:"prompt,omitempty"`
	Generated string    `json:"generated,omitempty"`
}

// NewSubAgentExecutor creates a SubAgentExecutor linked to a specific model and patcher.
func NewSubAgentExecutor(logger *slog.Logger, client ModelClient, modelName string, patcher workspace.Patcher) workspace.SubAgentExecutor {
	return func(ctx context.Context, bundle cognition.ThinkBundle, sandbox workspace.CandidateSandbox, agentID int, tokenChan chan<- workspace.AgentToken) error {

		if patcher != nil && patcher.SystemInstructions() != "" {
			bundle.Rules.FormattingConstraints = patcher.SystemInstructions()
		}

		logger.InfoContext(ctx, "sub-agent starting generation stream",
			"agent_id", agentID,
			"model", modelName,
			"has_token_chan", tokenChan != nil,
			"prompt_preview", truncateForLog(bundle.Trajectory.Catalyst.Content, 80),
			"workbench_files_count", len(bundle.Workbench),
		)

		// Sub-agent passes nil for tools and empty config: pure text/patch streaming
		stream := client.GenerateContentStream(ctx, modelName, bundle, nil, spaces.ThinkStateConfig{})

		rawOutput := ""
		chunkCount := 0
		var interceptedToolNames []string

		for chunk, err := range stream {
			if err != nil {
				logger.ErrorContext(ctx, "sub-agent stream failed with error",
					"agent_id", agentID,
					"model", modelName,
					"error", err,
				)
				return fmt.Errorf("sub-agent generation stream failed: %w", err)
			}

			chunkCount++

			if len(chunk.ToolCalls) > 0 {
				for _, tc := range chunk.ToolCalls {
					interceptedToolNames = append(interceptedToolNames, tc.Name)
				}
			}

			if chunk.Text != "" {
				rawOutput += chunk.Text

				if tokenChan != nil {
					tokenChan <- workspace.AgentToken{
						AgentID: agentID,
						Text:    chunk.Text,
					}
				}
			}
		}

		logger.DebugContext(ctx, "sub-agent stream completed",
			"agent_id", agentID,
			"model", modelName,
			"chunks_received", chunkCount,
			"text_bytes", len(rawOutput),
			"tool_calls_emitted", interceptedToolNames,
		)

		if rawOutput == "" {
			if len(interceptedToolNames) > 0 {
				logger.WarnContext(ctx, "sub-agent emitted tool calls instead of code/patch text",
					"agent_id", agentID,
					"model", modelName,
					"tool_calls", interceptedToolNames,
				)
				return fmt.Errorf("empty response from sub-agent: model emitted tool calls %v instead of patch text", interceptedToolNames)
			}
			return fmt.Errorf("empty response from sub-agent (received %d chunks)", chunkCount)
		}

		healer := GetHealerForModel(modelName)
		if healer != nil {
			healedOutput, modified := healer.Heal(rawOutput)
			if modified {
				logger.WarnContext(ctx, "model output healed",
					"model", modelName,
					"healer", fmt.Sprintf("%T", healer),
					"original", rawOutput,
					"healed", healedOutput)
				rawOutput = healedOutput
			}
		}

		if patcher != nil {
			if err := patcher.Apply(ctx, sandbox, rawOutput); err != nil {
				return fmt.Errorf("patcher failed to apply changes: %w\nRaw Output:\n%s", err, rawOutput)
			}
		}

		traceData, _ := sandbox.ReadFile(ctx, "trace.jsonl")
		event := TraceEvent{
			Timestamp: time.Now().UTC(),
			Prompt:    bundle.Trajectory.Catalyst.Content,
			Generated: rawOutput,
		}

		b, err := json.Marshal(event)
		if err == nil {
			traceData = append(traceData, b...)
			traceData = append(traceData, '\n')
			_ = sandbox.WriteFile(ctx, "trace.jsonl", traceData)
		}

		return nil
	}
}

func truncateForLog(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
