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
concept:
  type: "space"
  name: "concept"
fanout:
  type: "flow"
  name: "Parallel FanOut"
`),
		},
	}

	registry := config.NewRegistry()
	if err := registry.LoadFS(mockFS, "."); err != nil {
		t.Fatalf("LoadFS failed: %v", err)
	}

	// Test Config retrieval
	spaceCfg, ok := registry.GetConfig("golang")
	if !ok {
		t.Fatalf("GetConfig: Expected to find 'golang'")
	}
	if spaceCfg.Name != "Golang Environment" {
		t.Errorf("GetConfig: Expected Name 'Golang Environment', got %q", spaceCfg.Name)
	}

	// Test Domain instantiation mapping
	_, ok = registry.GetDomain("concept")
	if !ok {
		t.Errorf("GetDomain: Expected to find 'concept' domain instantiated")
	}

	// Test Flow retrieval
	flowCfg, ok := registry.GetFlow("fanout")
	if !ok {
		t.Errorf("GetFlow: Expected to find 'fanout'")
	}
	if flowCfg.Name != "Parallel FanOut" {
		t.Errorf("Expected flow name 'Parallel FanOut', got %q", flowCfg.Name)
	}
}
