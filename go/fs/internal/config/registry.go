package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// Registry holds all loaded ThinkSpace configurations and their initialized interfaces.
type Registry struct {
	spaces  map[string]workspace.ThinkSpace
	configs map[string]workspace.ThinkSpaceConfig
}

func NewRegistry() *Registry {
	return &Registry{
		spaces:  make(map[string]workspace.ThinkSpace),
		configs: make(map[string]workspace.ThinkSpaceConfig),
	}
}

// LoadDirectory scans the given path for .yaml files and loads them.
func (r *Registry) LoadDirectory(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read config directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("⚠️ Could not read %s: %v\n", path, err)
			continue
		}

		var cfg workspace.ThinkSpaceConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			fmt.Printf("⚠️ Invalid YAML at %s: %v\n", path, err)
			continue
		}

		cfg.ApplyDefaults()

		// The ID is simply the filename without the extension (e.g., "golang")
		id := strings.TrimSuffix(entry.Name(), ".yaml")

		r.spaces[id] = workspace.NewGoThinkSpace(cfg)
		r.configs[id] = cfg
	}

	return nil
}

// GetSpace retrieves an initialized ThinkSpace interface.
func (r *Registry) GetSpace(id string) (workspace.ThinkSpace, bool) {
	space, ok := r.spaces[id]
	return space, ok
}

// GetConfig retrieves the raw YAML configuration struct.
func (r *Registry) GetConfig(id string) (workspace.ThinkSpaceConfig, bool) {
	cfg, ok := r.configs[id]
	return cfg, ok
}

// GetAllConfigs returns a map of all loaded configurations (useful for the HTTP API).
func (r *Registry) GetAllConfigs() map[string]workspace.ThinkSpaceConfig {
	return r.configs
}

// GetAvailableSpaces returns a lightweight list for the WebSocket handshake.
func (r *Registry) GetAvailableSpaces() []struct {
	ID   string
	Name string
} {
	var list []struct {
		ID   string
		Name string
	}
	for id := range r.spaces {
		list = append(list, struct {
			ID   string
			Name string
		}{
			ID:   id,
			Name: id, // Can be updated if the YAML explicitly defines a display name later
		})
	}
	return list
}
