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
  models:
    manager: "gemini-3.6-flash"
  base_agent_rules: "Must use /src"

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

	_, ok = registry.GetDomain("golang")
	if !ok {
		t.Errorf("GetDomain: Expected to find 'golang'")
	}

	flowCfg, ok := registry.GetFlow("fanout")
	if !ok {
		t.Fatalf("GetFlow: Expected to find 'fanout'")
	}
	if flowCfg.Name != "Parallel FanOut" {
		t.Errorf("GetFlow: Expected Name 'Parallel FanOut', got %q", flowCfg.Name)
	}

	allConfigs := registry.GetAllConfigs()
	if len(allConfigs) != 1 {
		t.Errorf("GetAllConfigs: Expected 1 config, got %d", len(allConfigs))
	}
}
