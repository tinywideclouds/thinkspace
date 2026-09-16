package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"google.golang.org/genai"
)

type TraceEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Prompt    string    `json:"prompt,omitempty"`
	Generated string    `json:"generated,omitempty"`
}

// NewSubAgentExecutor creates a SubAgentExecutor linked to a specific model, system instruction, token limit, and patcher.
func NewSubAgentExecutor(logger *slog.Logger, client ModelClient, modelName string, systemInstruction string, maxTokens int, patcher workspace.Patcher) workspace.SubAgentExecutor {
	return func(ctx context.Context, briefing workspace.SubAgentBriefing, sandbox workspace.CandidateSandbox, agentID int, tokenChan chan<- workspace.AgentToken) error {

		combinedInstruction := systemInstruction
		if patcher != nil && patcher.SystemInstructions() != "" {
			combinedInstruction += "\n\n" + patcher.SystemInstructions()
		}

		var sysInstr *genai.Content
		if combinedInstruction != "" {
			sysInstr = &genai.Content{
				Parts: []*genai.Part{{Text: combinedInstruction}},
			}
		}

		config := &genai.GenerateContentConfig{
			SystemInstruction: sysInstr,
		}

		if maxTokens > 0 {
			config.MaxOutputTokens = int32(maxTokens)
		}

		var promptBuilder strings.Builder
		if briefing.ContextDigest != "" {
			promptBuilder.WriteString("Context:\n")
			promptBuilder.WriteString(briefing.ContextDigest)
			promptBuilder.WriteString("\n\n")
		}
		promptBuilder.WriteString("Task:\n")
		promptBuilder.WriteString(briefing.Instruction)

		fullPrompt := promptBuilder.String()

		contents := []*genai.Content{{
			Role:  "user",
			Parts: []*genai.Part{{Text: fullPrompt}},
		}}

		stream := client.GenerateContentStream(ctx, modelName, contents, config)

		rawOutput := ""

		for chunk, err := range stream {
			if err != nil {
				return fmt.Errorf("sub-agent generation stream failed: %w", err)
			}

			if len(chunk.Candidates) > 0 && chunk.Candidates[0].Content != nil {
				for _, part := range chunk.Candidates[0].Content.Parts {
					if part.Text != "" {
						rawOutput += part.Text

						if tokenChan != nil {
							tokenChan <- workspace.AgentToken{
								AgentID: agentID,
								Text:    part.Text,
							}
						}
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
			Prompt:    fullPrompt,
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
