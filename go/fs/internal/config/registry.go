package config

import (
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace/spaces/golang"
)

type Registry struct {
	spaces  map[string]workspace.ThinkSpace
	configs map[string]workspace.ThinkSpaceConfig
	flows   map[string]flows.FlowConfig
}

func NewRegistry() *Registry {
	return &Registry{
		spaces:  make(map[string]workspace.ThinkSpace),
		configs: make(map[string]workspace.ThinkSpaceConfig),
		flows:   make(map[string]flows.FlowConfig),
	}
}

// LoadDirectory is a convenience wrapper for the physical filesystem.
func (r *Registry) LoadDirectory(dir string) error {
	return r.LoadFS(os.DirFS(dir), ".")
}

// LoadFS traverses the given filesystem recursively and builds the master registry.
func (r *Registry) LoadFS(fileSystem fs.FS, root string) error {
	return fs.WalkDir(fileSystem, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".yaml") {
			return nil
		}

		data, err := fs.ReadFile(fileSystem, path)
		if err != nil {
			fmt.Printf("⚠️ Could not read %s: %v\n", path, err)
			return nil
		}

		parsed, err := ParseConfigBytes(data)
		if err != nil {
			fmt.Printf("⚠️ Config error in %s: %v\n", path, err)
			return nil
		}

		// Instantiate successfully parsed blocks
		for id, spaceCfg := range parsed.Spaces {
			r.spaces[id] = golang.NewGoThinkSpace(spaceCfg)
			r.configs[id] = spaceCfg
		}
		for id, flowCfg := range parsed.Flows {
			r.flows[id] = flowCfg
		}

		return nil
	})
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

// GetFlow retrieves a loaded flow configuration.
func (r *Registry) GetFlow(id string) (flows.FlowConfig, bool) {
	cfg, ok := r.flows[id]
	return cfg, ok
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
			Name: id,
		})
	}
	return list
}

// GetAllConfigs returns a map of all loaded space configurations (used by the HTTP API).
func (r *Registry) GetAllConfigs() map[string]workspace.ThinkSpaceConfig {
	return r.configs
}

// GetAllFlows returns a map of all loaded flow configurations (used by the HTTP API).
func (r *Registry) GetAllFlows() map[string]flows.FlowConfig {
	return r.flows
}
