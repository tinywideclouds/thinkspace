package config_test

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// mockRegistryThinkSpace is used to verify manual DI injection
type mockRegistryThinkSpace struct {
	cfg spaces.ThinkStateConfig
}

func (m *mockRegistryThinkSpace) Config() spaces.ThinkStateConfig                        { return m.cfg }
func (m *mockRegistryThinkSpace) Tools() []llm.ToolDef                                   { return nil }
func (m *mockRegistryThinkSpace) Verifier() spaces.Verifier                              { return nil }
func (m *mockRegistryThinkSpace) Mapbook() spaces.Mapbook                                { return nil }
func (m *mockRegistryThinkSpace) Patcher() workspace.Patcher                             { return nil }
func (m *mockRegistryThinkSpace) Initialize(ctx context.Context, directory string) error { return nil }
func (m *mockRegistryThinkSpace) EnrichBundle(ctx context.Context, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus, workspaceRoot string) error {
	return nil
}

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

func TestRegistry_ManualRegistration(t *testing.T) {
	registry := config.NewRegistry()

	cfg := spaces.ThinkStateConfig{Name: "Mock Domain"}
	mockSpace := &mockRegistryThinkSpace{cfg: cfg}

	// Explicitly register the domain and config (used in tests)
	registry.RegisterDomain("mock-domain", mockSpace)
	registry.RegisterConfig("mock-domain", cfg)

	// Verify Domain Injection
	retrievedSpace, ok := registry.GetDomain("mock-domain")
	if !ok {
		t.Fatalf("Expected to find manually registered domain")
	}
	if retrievedSpace.Config().Name != "Mock Domain" {
		t.Errorf("Expected domain name 'Mock Domain', got '%s'", retrievedSpace.Config().Name)
	}

	// Verify Config Injection
	retrievedCfg, ok := registry.GetConfig("mock-domain")
	if !ok {
		t.Fatalf("Expected to find manually registered config")
	}
	if retrievedCfg.Name != "Mock Domain" {
		t.Errorf("Expected config name 'Mock Domain', got '%s'", retrievedCfg.Name)
	}
}
