package config_test

import (
	"testing"
	"testing/fstest"

	"github.com/tinywideclouds.com/thinkspace/internal/config"
)

func TestRegistry_Methods(t *testing.T) {
	mockFS := fstest.MapFS{
		"config.yaml": &fstest.MapFile{
			Data: []byte(`
golang:
  type: "space"
  name: "Golang Environment"
  system_prompt: "Base rules"
  roles:
    manager:
      model: "gemini-3.6-flash"
      tools:
        propose_change:
          description: "mock tool"
          agent_count_description: "mock count"
          agent_instructions_description: "mock inst array"
          context_digest_description: "mock digest"
          instruction_description: "mock instruction"
    worker:
      max_tokens: 8192
      retry_prompt: "mock retry prompt"
      system_prompt: "Must use /src"

fanout:
  type: "flow"
  name: "Parallel FanOut"
  retry_prompt: "Please review the trace..."
`),
		},
	}

	registry := config.NewRegistry()
	if err := registry.LoadFS(mockFS, "."); err != nil {
		t.Fatalf("LoadFS failed: %v", err)
	}

	spaceCfg, ok := registry.GetConfig("golang")
	if !ok {
		t.Fatalf("GetConfig: Expected to find 'golang'")
	}
	if spaceCfg.Name != "Golang Environment" {
		t.Errorf("GetConfig: Expected Name 'Golang Environment', got %q", spaceCfg.Name)
	}
	if spaceCfg.SystemPrompt != "Base rules" {
		t.Errorf("GetConfig: Expected SystemPrompt 'Base rules', got %q", spaceCfg.SystemPrompt)
	}
	if spaceCfg.Roles.Worker.SystemPrompt != "Must use /src" {
		t.Errorf("GetConfig: Expected Worker SystemPrompt 'Must use /src', got %q", spaceCfg.Roles.Worker.SystemPrompt)
	}
	if spaceCfg.Roles.Worker.RetryPrompt != "mock retry prompt" {
		t.Errorf("GetConfig: Expected WorkerRetryPrompt 'mock retry prompt', got %q", spaceCfg.Roles.Worker.RetryPrompt)
	}
}
