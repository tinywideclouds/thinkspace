package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
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

		// Inject physical formatting laws into the cognitive rules
		if patcher != nil && patcher.SystemInstructions() != "" {
			bundle.Rules.FormattingConstraints = patcher.SystemInstructions()
		}

		stream := client.GenerateContentStream(ctx, modelName, bundle, nil)

		rawOutput := ""
		for chunk, err := range stream {
			if err != nil {
				return fmt.Errorf("sub-agent generation stream failed: %w", err)
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

		if rawOutput == "" {
			return fmt.Errorf("empty response from sub-agent")
		}

		// Apply Model-Specific Output Healers
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
