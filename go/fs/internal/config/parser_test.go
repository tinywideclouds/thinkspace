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
  models:
    manager: "gemini-3.6-flash"
  base_agent_rules: "Must use /src"

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

	flowCfg, ok := parsed.Flows["fanout"]
	if !ok {
		t.Fatalf("Expected to find 'fanout' flow config")
	}
	if flowCfg.Name != "Parallel FanOut" {
		t.Errorf("Expected Flow Name 'Parallel FanOut', got %q", flowCfg.Name)
	}
}

func TestParseConfigBytes_InvalidType(t *testing.T) {
	yamlData := []byte(`
bad-block:
  type: "unknown-type"
`)

	_, err := config.ParseConfigBytes(yamlData)
	if err == nil {
		t.Fatalf("Expected error for unknown config type, got nil")
	}
}
