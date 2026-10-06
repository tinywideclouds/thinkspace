package config_test

import (
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
  type: "space"
  name: "Golang Environment"
  system_prompt: "Base rules"
  roles:
    worker:
      max_tokens: 8192
      retry_prompt: "mock retry prompt"
      system_prompt: "Must use /src"
`,
			validate: func(t *testing.T, parsed *config.ParsedConfig) {
				cfg, ok := parsed.Spaces["golang"]
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
			name: "Concept Space with Create Digest",
			yaml: `
concept:
  type: "space"
  name: "Concept Environment"
  roles:
    manager:
      tools:
        create_digest:
          description: "mock digest description"
          topic_name_description: "mock topic"
`,
			validate: func(t *testing.T, parsed *config.ParsedConfig) {
				cfg, ok := parsed.Spaces["concept"]
				if !ok {
					t.Fatalf("missing concept space")
				}
				if cfg.Roles.Manager.Tools.CreateDigest.Description != "mock digest description" {
					t.Errorf("expected 'mock digest description', got %q", cfg.Roles.Manager.Tools.CreateDigest.Description)
				}
			},
		},
		{
			name: "Flow Config",
			yaml: `
fanout:
  type: "flow"
  name: "Parallel FanOut"
`,
			validate: func(t *testing.T, parsed *config.ParsedConfig) {
				cfg, ok := parsed.Flows["fanout"]
				if !ok {
					t.Fatalf("missing fanout flow")
				}
				if cfg.Name != "Parallel FanOut" {
					t.Errorf("expected 'Parallel FanOut', got %q", cfg.Name)
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
