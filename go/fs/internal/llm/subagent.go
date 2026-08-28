package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"google.golang.org/genai"
)

// TraceEvent represents a line in the shadow ledger for the sub-agent.
type TraceEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Prompt    string    `json:"prompt,omitempty"`
	Generated string    `json:"generated,omitempty"`
}

// SubAgentFactory creates a SubAgentExecutor linked to a specific model.
func SubAgentFactory(client ModelClient, modelName string) workspace.SubAgentExecutor {
	// The signature now strictly matches workspace.SubAgentExecutor using the virtual sandbox
	return func(ctx context.Context, instructions string, sandbox workspace.CandidateSandbox, agentID int, tokenChan chan<- workspace.AgentToken) error {

		config := &genai.GenerateContentConfig{
			ResponseMIMEType: "application/json",
		}

		contents := []*genai.Content{{
			Role:  "user",
			Parts: []*genai.Part{{Text: instructions}},
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

		var files map[string]string
		if err := json.Unmarshal([]byte(rawJSON), &files); err != nil {
			return fmt.Errorf("failed to parse sub-agent json: %w\nOutput was: %s", err, rawJSON)
		}

		// Write the generated files into the virtual sandbox environment
		for relPath, content := range files {
			cleanPath := filepath.Clean(relPath)
			if err := sandbox.WriteFile(ctx, cleanPath, []byte(content)); err != nil {
				return fmt.Errorf("writing file %s: %w", relPath, err)
			}
		}

		// Read the existing trace ledger (if any) and append the new event
		traceData, _ := sandbox.ReadFile(ctx, "trace.jsonl")

		event := TraceEvent{
			Timestamp: time.Now().UTC(),
			Prompt:    instructions,
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
