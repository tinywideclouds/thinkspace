package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces/golang"
)

type Registry struct {
	mu      sync.RWMutex
	domains map[string]spaces.ThinkSpace
	configs map[string]spaces.ThinkSpaceConfig
	flows   map[string]flows.FlowConfig
}

func NewRegistry() *Registry {
	return &Registry{
		domains: make(map[string]spaces.ThinkSpace),
		configs: make(map[string]spaces.ThinkSpaceConfig),
		flows:   make(map[string]flows.FlowConfig),
	}
}

func (r *Registry) LoadDirectory(dir string) error {
	return r.LoadFS(os.DirFS(dir), ".")
}

func (r *Registry) LoadFS(fsys fs.FS, dir string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			continue
		}

		parsed, err := ParseConfigBytes(data)
		if err != nil {
			continue
		}

		for k, v := range parsed.Spaces {
			r.configs[k] = v
			r.domains[k] = golang.NewGoThinkSpace(v)
		}
		for k, v := range parsed.Flows {
			r.flows[k] = v
		}
	}
	return nil
}

func (r *Registry) GetDomain(id string) (spaces.ThinkSpace, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	domain, ok := r.domains[id]
	return domain, ok
}

func (r *Registry) GetConfig(id string) (spaces.ThinkSpaceConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cfg, ok := r.configs[id]
	return cfg, ok
}

func (r *Registry) GetFlow(id string) (flows.FlowConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	flow, ok := r.flows[id]
	return flow, ok
}

func (r *Registry) GetAllConfigs() map[string]spaces.ThinkSpaceConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make(map[string]spaces.ThinkSpaceConfig)
	for k, v := range r.configs {
		res[k] = v
	}
	return res
}
