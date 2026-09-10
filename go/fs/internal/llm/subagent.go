package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
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

// NewSubAgentExecutor creates a SubAgentExecutor linked to a specific model, system instruction, and token limit.
func NewSubAgentExecutor(client ModelClient, modelName string, systemInstruction string, maxTokens int) workspace.SubAgentExecutor {
	return func(ctx context.Context, briefing workspace.SubAgentBriefing, sandbox workspace.CandidateSandbox, agentID int, tokenChan chan<- workspace.AgentToken) error {

		var sysInstr *genai.Content
		if systemInstruction != "" {
			sysInstr = &genai.Content{
				Parts: []*genai.Part{{Text: systemInstruction}},
			}
		}

		config := &genai.GenerateContentConfig{
			ResponseMIMEType:  "application/json",
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

		rawJSON := ""

		for chunk, err := range stream {
			if err != nil {
				return fmt.Errorf("sub-agent generation stream failed: %w", err)
			}

			if len(chunk.Candidates) > 0 && chunk.Candidates[0].Content != nil {
				for _, part := range chunk.Candidates[0].Content.Parts {
					if part.Text != "" {
						rawJSON += part.Text

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

		if rawJSON == "" {
			return fmt.Errorf("empty response from sub-agent")
		}

		// Sanitize hallucinated JSON escapes (e.g., \), \:) before unmarshaling
		re := regexp.MustCompile(`\\([^"\\/bfnrtu])`)
		sanitizedJSON := re.ReplaceAllString(rawJSON, "$1")

		var files map[string]string
		if err := json.Unmarshal([]byte(sanitizedJSON), &files); err != nil {
			return fmt.Errorf("failed to parse sub-agent json: %w\nOutput was: %s", err, rawJSON)
		}

		for relPath, content := range files {
			// Normalize to forward slashes to prevent cross-platform OS bugs (like Windows \ paths)
			cleanPath := filepath.ToSlash(filepath.Clean(relPath))
			if err := sandbox.WriteFile(ctx, cleanPath, []byte(content)); err != nil {
				return fmt.Errorf("writing file %s: %w", relPath, err)
			}
		}

		traceData, _ := sandbox.ReadFile(ctx, "trace.jsonl")

		event := TraceEvent{
			Timestamp: time.Now().UTC(),
			Prompt:    fullPrompt,
			Generated: rawJSON,
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
