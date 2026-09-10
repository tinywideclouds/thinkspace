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
  max_worker_tokens: 8192
  tool_description: "mock tool"
  agent_count_description: "mock count"
  agent_instructions_description: "mock inst array"
  context_digest_description: "mock digest"
  instruction_description: "mock instruction"
  worker_retry_prompt: "mock retry prompt"
  models:
    manager: "gemini-3.6-flash"
  roles:
    worker: "Must use /src"

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
	if spaceCfg.Roles.Worker != "Must use /src" {
		t.Errorf("GetConfig: Expected Worker Role 'Must use /src', got %q", spaceCfg.Roles.Worker)
	}
	if spaceCfg.WorkerRetryPrompt != "mock retry prompt" {
		t.Errorf("GetConfig: Expected WorkerRetryPrompt 'mock retry prompt', got %q", spaceCfg.WorkerRetryPrompt)
	}
}
