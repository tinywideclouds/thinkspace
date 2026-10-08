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
digest:
  type: "core"
  name: "Digest Manager"
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

	// Test Flow decoding
	type mockConfig struct {
		Name string `yaml:"name"`
	}
	var flowCfg mockConfig
	if err := registry.DecodeFlowConfig("fanout", &flowCfg); err != nil {
		t.Fatalf("DecodeFlowConfig failed: %v", err)
	}
	if flowCfg.Name != "Parallel FanOut" {
		t.Errorf("Expected flow name 'Parallel FanOut', got %q", flowCfg.Name)
	}

	// Test Core decoding
	var coreCfg mockConfig
	if err := registry.DecodeCoreConfig("digest", &coreCfg); err != nil {
		t.Fatalf("DecodeCoreConfig failed: %v", err)
	}
	if coreCfg.Name != "Digest Manager" {
		t.Errorf("Expected core name 'Digest Manager', got %q", coreCfg.Name)
	}
}
