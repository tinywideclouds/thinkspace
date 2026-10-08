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
`,
			validate: func(t *testing.T, parsed *config.ParsedConfig) {
				node, ok := parsed.Flows["fanout"]
				if !ok {
					t.Fatalf("missing fanout flow")
				}

				var cfg struct {
					Name string `yaml:"name"`
				}
				if err := node.Decode(&cfg); err != nil {
					t.Fatalf("failed to decode node: %v", err)
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
