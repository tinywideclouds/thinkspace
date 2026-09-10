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
	if spaceCfg.Roles.Worker != "Must use /src" {
		t.Errorf("Expected Worker Role to be 'Must use /src', got %q", spaceCfg.Roles.Worker)
	}
	if spaceCfg.MaxWorkerTokens != 8192 {
		t.Errorf("Expected MaxWorkerTokens to be 8192, got %d", spaceCfg.MaxWorkerTokens)
	}
	if spaceCfg.ToolDescription != "mock tool" {
		t.Errorf("Expected ToolDescription to be 'mock tool', got %q", spaceCfg.ToolDescription)
	}
	if spaceCfg.WorkerRetryPrompt != "mock retry prompt" {
		t.Errorf("Expected WorkerRetryPrompt to be 'mock retry prompt', got %q", spaceCfg.WorkerRetryPrompt)
	}

	flowCfg, ok := parsed.Flows["fanout"]
	if !ok {
		t.Fatalf("Expected to find 'fanout' flow config")
	}
	if flowCfg.Name != "Parallel FanOut" {
		t.Errorf("Expected Flow Name 'Parallel FanOut', got %q", flowCfg.Name)
	}
}
