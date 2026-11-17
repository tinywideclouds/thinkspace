package config_test

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// mockRegistryThinkSpace is used to verify manual DI injection
type mockRegistryThinkSpace struct {
	cfg spaces.ThinkStateConfig
}

func (m *mockRegistryThinkSpace) Config() spaces.ThinkStateConfig                        { return m.cfg }
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
  type: "thinkstate"
  name: "Golang Environment"
concept:
  type: "thinkstate"
  name: "concept"
fanout:
  type: "flow"
  name: "Parallel FanOut"
  retry_prompt: "Retry"
  review_prompt: "Review"
  refine_prompt: "Refine"
  synthesize_prompt: "Synthesize"
  synthesis_review_prompt: "Summary"
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

	// Test ThinkState instantiation mapping
	_, ok = registry.GetThinkState("concept")
	if !ok {
		t.Errorf("GetThinkState: Expected to find 'concept' think state instantiated")
	}

	// Test Flow decoding
	flowCfg, err := registry.GetFanOutConfig()
	if err != nil {
		t.Fatalf("GetFanOutConfig failed: %v", err)
	}
	if flowCfg.Name != "Parallel FanOut" {
		t.Errorf("Expected flow name 'Parallel FanOut', got %q", flowCfg.Name)
	}

	// Test Core decoding
	type mockConfig struct {
		Name string `yaml:"name"`
	}
	var coreCfg mockConfig
	if err := registry.DecodeCoreConfig("digest", &coreCfg); err != nil {
		t.Fatalf("DecodeCoreConfig failed: %v", err)
	}
	if coreCfg.Name != "Digest Manager" {
		t.Errorf("Expected core name 'Digest Manager', got %q", coreCfg.Name)
	}
}

func TestRegistry_LoadFS_ReportsInvalidFlowWithoutBlockingSubsequentThinkStates(t *testing.T) {
	mockFS := fstest.MapFS{
		"fanout.yaml": &fstest.MapFile{
			Data: []byte(`
fanout:
  type: "flow"
  name: "Parallel FanOut"
  retry_prompt: "Retry"
`),
		},
		"golang.yaml": &fstest.MapFile{
			Data: []byte(`
golang:
  type: "thinkstate"
  name: "Golang Environment"
`),
		},
		"intake.yaml": &fstest.MapFile{
			Data: []byte(`
intake:
  type: "thinkstate"
  name: "Intake"
`),
		},
	}

	registry := config.NewRegistry()
	err := registry.LoadFS(mockFS, ".")
	if err == nil {
		t.Fatalf("expected LoadFS to return error for invalid fanout config, got nil")
	}
	if !strings.Contains(err.Error(), "fanout.yaml") {
		t.Errorf("expected error to identify failing file fanout.yaml, got: %v", err)
	}

	if _, ok := registry.GetThinkState("golang"); !ok {
		t.Errorf("expected 'golang' think state to still load when fanout.yaml fails validation")
	}
	if _, ok := registry.GetThinkState("intake"); !ok {
		t.Errorf("expected 'intake' think state to still load when fanout.yaml fails validation")
	}
}

func TestRegistry_ManualRegistration(t *testing.T) {
	registry := config.NewRegistry()

	cfg := spaces.ThinkStateConfig{Name: "Mock Domain"}
	mockSpace := &mockRegistryThinkSpace{cfg: cfg}

	// Explicitly register the think state and config (used in tests)
	registry.RegisterThinkState("mock-domain", mockSpace)
	registry.RegisterConfig("mock-domain", cfg)

	// Verify ThinkState Injection
	retrievedSpace, ok := registry.GetThinkState("mock-domain")
	if !ok {
		t.Fatalf("Expected to find manually registered think state")
	}
	if retrievedSpace.Config().Name != "Mock Domain" {
		t.Errorf("Expected think state name 'Mock Domain', got '%s'", retrievedSpace.Config().Name)
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
