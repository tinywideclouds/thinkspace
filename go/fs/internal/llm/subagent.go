package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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
func SubAgentFactory(client *genai.Client, modelName string) workspace.SubAgentExecutor {
	// The signature now strictly matches workspace.SubAgentExecutor
	return func(ctx context.Context, instructions string, sandboxDir string, agentID int, tokenChan chan<- workspace.AgentToken) error {

		config := &genai.GenerateContentConfig{
			ResponseMIMEType: "application/json",
		}

		contents := []*genai.Content{{
			Role:  "user",
			Parts: []*genai.Part{{Text: instructions}},
		}}

		// Upgraded to a streaming call
		stream := client.Models.GenerateContentStream(ctx, modelName, contents, config)

		rawJSON := ""

		// Consume the stream and multiplex it out
		for chunk, err := range stream {
			if err != nil {
				return fmt.Errorf("sub-agent generation stream failed: %w", err)
			}

			if len(chunk.Candidates) > 0 && chunk.Candidates[0].Content != nil {
				for _, part := range chunk.Candidates[0].Content.Parts {
					if part.Text != "" {
						rawJSON += part.Text

						// Push the token to the multiplex channel (if connected)
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

		// Write the generated files into the sandbox
		for relPath, content := range files {
			absPath := filepath.Join(sandboxDir, filepath.Clean(relPath))
			if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
				return fmt.Errorf("creating dir for %s: %w", relPath, err)
			}
			if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
				return fmt.Errorf("writing file %s: %w", relPath, err)
			}
		}

		// Write the shadow ledger directly to the root of the sandbox
		tracePath := filepath.Join(sandboxDir, "trace.jsonl")
		traceFile, err := os.OpenFile(tracePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open trace file: %w", err)
		}
		defer traceFile.Close()

		event := TraceEvent{
			Timestamp: time.Now().UTC(),
			Prompt:    instructions,
			Generated: rawJSON,
		}

		b, err := json.Marshal(event)
		if err == nil {
			_, _ = traceFile.Write(append(b, '\n'))
		}

		return nil
	}
}
