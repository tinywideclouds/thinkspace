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
`),
		},
	}

	registry := config.NewRegistry()
	if err := registry.LoadFS(mockFS, "."); err != nil {
		t.Fatalf("LoadFS failed: %v", err)
	}

	// 1. Test GetConfig & GetSpace
	spaceCfg, ok := registry.GetConfig("golang-space")
	if !ok {
		t.Fatalf("GetConfig: Expected to find 'golang-space'")
	}
	if spaceCfg.Name != "Golang Environment" {
		t.Errorf("GetConfig: Expected Name 'Golang Environment', got %q", spaceCfg.Name)
	}

	_, ok = registry.GetSpace("golang-space")
	if !ok {
		t.Errorf("GetSpace: Expected to find 'golang-space'")
	}

	// 2. Test GetFlow
	flowCfg, ok := registry.GetFlow("fanout")
	if !ok {
		t.Fatalf("GetFlow: Expected to find 'fanout'")
	}
	if flowCfg.Name != "Parallel FanOut" {
		t.Errorf("GetFlow: Expected Name 'Parallel FanOut', got %q", flowCfg.Name)
	}

	// 3. Test GetAllConfigs
	allConfigs := registry.GetAllConfigs()
	if len(allConfigs) != 1 {
		t.Errorf("GetAllConfigs: Expected 1 config, got %d", len(allConfigs))
	}
	if _, exists := allConfigs["golang-space"]; !exists {
		t.Errorf("GetAllConfigs: Missing 'golang-space'")
	}

	// 4. Test GetAllFlows
	allFlows := registry.GetAllFlows()
	if len(allFlows) != 1 {
		t.Errorf("GetAllFlows: Expected 1 flow, got %d", len(allFlows))
	}
	if _, exists := allFlows["fanout"]; !exists {
		t.Errorf("GetAllFlows: Missing 'fanout'")
	}

	// 5. Test GetAvailableSpaces
	availableSpaces := registry.GetAvailableSpaces()
	if len(availableSpaces) != 1 {
		t.Fatalf("GetAvailableSpaces: Expected 1 space, got %d", len(availableSpaces))
	}
	if availableSpaces[0].ID != "golang-space" {
		t.Errorf("GetAvailableSpaces: Expected ID 'golang-space', got %q", availableSpaces[0].ID)
	}
}
