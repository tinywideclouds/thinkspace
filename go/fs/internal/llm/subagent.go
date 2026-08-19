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
	return func(ctx context.Context, instructions string, sandboxDir string) error {

		config := &genai.GenerateContentConfig{
			ResponseMIMEType: "application/json",
		}

		contents := []*genai.Content{{
			Role:  "user",
			Parts: []*genai.Part{{Text: instructions}},
		}}

		resp, err := client.Models.GenerateContent(ctx, modelName, contents, config)
		if err != nil {
			return fmt.Errorf("sub-agent generation failed: %w", err)
		}

		if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
			return fmt.Errorf("empty response from sub-agent")
		}

		rawJSON := ""
		for _, part := range resp.Candidates[0].Content.Parts {
			rawJSON += part.Text
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
