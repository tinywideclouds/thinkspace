package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/tinywideclouds.com/thinkspace/internal/config"
)

func TestDefaultYAMLs_LoadAndValidate(t *testing.T) {
	entries, err := os.ReadDir("defaults")
	if err != nil {
		t.Fatalf("failed to read defaults directory: %v", err)
	}

	configFS := fstest.MapFS{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") || entry.Name() == "skeleton.yaml" {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join("defaults", entry.Name()))
		if readErr != nil {
			t.Fatalf("failed to read defaults/%s: %v", entry.Name(), readErr)
		}
		configFS[entry.Name()] = &fstest.MapFile{Data: data}
	}

	registry := config.NewRegistry()
	if err := registry.LoadFS(configFS, "."); err != nil {
		t.Fatalf("failed to load and validate default YAML configs in internal/config/defaults: %v", err)
	}

	fanoutCfg, err := registry.GetFanOutConfig()
	if err != nil {
		t.Fatalf("default fanout.yaml failed validation: %v", err)
	}
	if fanoutCfg.Version < 3 {
		t.Errorf("expected default fanout.yaml version >= 3, got %d", fanoutCfg.Version)
	}

	for _, requiredState := range []string{"intake", "golang"} {
		ts, ok := registry.GetThinkState(requiredState)
		if !ok || ts == nil {
			t.Errorf("expected default ThinkState %q to be loaded from defaults/", requiredState)
			continue
		}
		cfg, cfgOk := registry.GetConfig(requiredState)
		if !cfgOk {
			t.Errorf("expected config for default ThinkState %q", requiredState)
			continue
		}
		if strings.TrimSpace(cfg.Name) == "" {
			t.Errorf("default ThinkState %q is missing 'name'", requiredState)
		}
		if strings.TrimSpace(cfg.SystemPrompt) == "" {
			t.Errorf("default ThinkState %q is missing 'system_prompt'", requiredState)
		}
	}

	var digestCfg struct {
		Name string `yaml:"name"`
	}
	if err := registry.DecodeCoreConfig("digest", &digestCfg); err != nil {
		t.Errorf("expected default digest core config to decode cleanly: %v", err)
	}
}
