package config_test

import (
	"strings"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/config"
)

func TestParseConfigBytes(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		validate func(t *testing.T, parsed *config.ParsedConfig)
	}{
		{
			name: "Golang Space",
			yaml: `
golang:
  type: "thinkstate"
  name: "Golang Environment"
  system_prompt: "Base rules"
  roles:
    worker:
      max_tokens: 8192
      retry_prompt: "mock retry prompt"
      system_prompt: "Must use /src"
`,
			validate: func(t *testing.T, parsed *config.ParsedConfig) {
				cfg, ok := parsed.ThinkStates["golang"]
				if !ok {
					t.Fatalf("missing golang space")
				}
				if cfg.Name != "Golang Environment" {
					t.Errorf("expected 'Golang Environment', got %q", cfg.Name)
				}
				if cfg.Roles.Worker.MaxTokens != 8192 {
					t.Errorf("expected 8192 max tokens, got %d", cfg.Roles.Worker.MaxTokens)
				}
			},
		},
		{
			name: "Core System Config",
			yaml: `
digest:
  type: "core"
  name: "Digest Manager"
`,
			validate: func(t *testing.T, parsed *config.ParsedConfig) {
				node, ok := parsed.Core["digest"]
				if !ok {
					t.Fatalf("missing digest core config")
				}

				var cfg struct {
					Name string `yaml:"name"`
				}
				if err := node.Decode(&cfg); err != nil {
					t.Fatalf("failed to decode node: %v", err)
				}

				if cfg.Name != "Digest Manager" {
					t.Errorf("expected 'Digest Manager', got %q", cfg.Name)
				}
			},
		},
		{
			name: "Flow Config",
			yaml: `
fanout:
  type: "flow"
  name: "Parallel FanOut"
  retry_prompt: "Retry: {{.ErrorTrace}}"
  review_prompt: "Review: {{ range .Branches }}{{.CandidateID}}{{ end }}"
  refine_prompt: "Refine: {{ range .Branches }}{{.CandidateID}}{{ end }}"
  synthesize_prompt: "Synthesize best version."
  synthesis_review_prompt: "Summary of {{ .CandidateID }}"
`,
			validate: func(t *testing.T, parsed *config.ParsedConfig) {
				node, ok := parsed.Flows["fanout"]
				if !ok {
					t.Fatalf("missing fanout flow")
				}

				var cfg config.FanOutFlowConfig
				if err := node.Decode(&cfg); err != nil {
					t.Fatalf("failed to decode node: %v", err)
				}

				if cfg.Name != "Parallel FanOut" {
					t.Errorf("expected 'Parallel FanOut', got %q", cfg.Name)
				}
				if cfg.SynthesizePrompt != "Synthesize best version." {
					t.Errorf("expected SynthesizePrompt 'Synthesize best version.', got %q", cfg.SynthesizePrompt)
				}
				if cfg.SynthesisReviewPrompt != "Summary of {{ .CandidateID }}" {
					t.Errorf("expected SynthesisReviewPrompt 'Summary of {{ .CandidateID }}', got %q", cfg.SynthesisReviewPrompt)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := config.ParseConfigBytes([]byte(tt.yaml))
			if err != nil {
				t.Fatalf("unexpected error parsing yaml: %v", err)
			}
			tt.validate(t, parsed)
		})
	}
}

func TestParseConfigBytes_FanOutValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		expectedErr string
	}{
		{
			name: "Missing refine_prompt",
			yaml: `
fanout:
  type: "flow"
  name: "Parallel FanOut"
  retry_prompt: "Retry"
  review_prompt: "Review"
  synthesize_prompt: "Synthesize"
  synthesis_review_prompt: "Summary"
`,
			expectedErr: "refine_prompt",
		},
		{
			name: "Missing synthesize_prompt",
			yaml: `
fanout:
  type: "flow"
  name: "Parallel FanOut"
  retry_prompt: "Retry"
  review_prompt: "Review"
  refine_prompt: "Refine"
  synthesis_review_prompt: "Summary"
`,
			expectedErr: "synthesize_prompt",
		},
		{
			name: "Missing synthesis_review_prompt",
			yaml: `
fanout:
  type: "flow"
  name: "Parallel FanOut"
  retry_prompt: "Retry"
  review_prompt: "Review"
  refine_prompt: "Refine"
  synthesize_prompt: "Synthesize"
`,
			expectedErr: "synthesis_review_prompt",
		},
		{
			name: "Malformed template syntax in refine_prompt",
			yaml: `
fanout:
  type: "flow"
  name: "Parallel FanOut"
  retry_prompt: "Retry"
  review_prompt: "Review"
  refine_prompt: "Refine {{ range .Branches "
  synthesize_prompt: "Synthesize"
  synthesis_review_prompt: "Summary"
`,
			expectedErr: "refine_prompt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := config.ParseConfigBytes([]byte(tt.yaml))
			if err == nil {
				t.Fatalf("expected ParseConfigBytes to fail validation, got nil")
			}
			if !strings.Contains(err.Error(), tt.expectedErr) {
				t.Errorf("expected error mentioning %q, got: %v", tt.expectedErr, err)
			}
		})
	}
}
