package config_test

import (
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/config"
)

func TestParseConfigBytes_Success(t *testing.T) {
	yamlData := []byte(`
golang-space:
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
`)

	parsed, err := config.ParseConfigBytes(yamlData)
	if err != nil {
		t.Fatalf("ParseConfigBytes failed: %v", err)
	}

	spaceCfg, ok := parsed.Spaces["golang-space"]
	if !ok {
		t.Fatalf("Expected to find 'golang-space' config")
	}
	if spaceCfg.Name != "Golang Environment" {
		t.Errorf("Expected Name 'Golang Environment', got %q", spaceCfg.Name)
	}
	if spaceCfg.SystemPrompt != "Base rules" {
		t.Errorf("Expected SystemPrompt to be 'Base rules', got %q", spaceCfg.SystemPrompt)
	}
	if spaceCfg.Roles.Worker.SystemPrompt != "Must use /src" {
		t.Errorf("Expected Worker SystemPrompt to be 'Must use /src', got %q", spaceCfg.Roles.Worker.SystemPrompt)
	}
	if spaceCfg.Roles.Worker.MaxTokens != 8192 {
		t.Errorf("Expected MaxWorkerTokens to be 8192, got %d", spaceCfg.Roles.Worker.MaxTokens)
	}
	if spaceCfg.Roles.Manager.Tools.ProposeChange.Description != "mock tool" {
		t.Errorf("Expected ToolDescription to be 'mock tool', got %q", spaceCfg.Roles.Manager.Tools.ProposeChange.Description)
	}
	if spaceCfg.Roles.Worker.RetryPrompt != "mock retry prompt" {
		t.Errorf("Expected WorkerRetryPrompt to be 'mock retry prompt', got %q", spaceCfg.Roles.Worker.RetryPrompt)
	}

	flowCfg, ok := parsed.Flows["fanout"]
	if !ok {
		t.Fatalf("Expected to find 'fanout' flow config")
	}
	if flowCfg.Name != "Parallel FanOut" {
		t.Errorf("Expected Flow Name 'Parallel FanOut', got %q", flowCfg.Name)
	}
}
