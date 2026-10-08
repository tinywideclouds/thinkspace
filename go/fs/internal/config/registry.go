package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces/golang"
)

type Registry struct {
	mu      sync.RWMutex
	domains map[string]spaces.ThinkSpace
	configs map[string]spaces.ThinkSpaceConfig
	flows   map[string]yaml.Node
	core    map[string]yaml.Node
}

func NewRegistry() *Registry {
	return &Registry{
		domains: make(map[string]spaces.ThinkSpace),
		configs: make(map[string]spaces.ThinkSpaceConfig),
		flows:   make(map[string]yaml.Node),
		core:    make(map[string]yaml.Node),
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
		for k, v := range parsed.Core {
			r.core[k] = v
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

func (r *Registry) DecodeFlowConfig(id string, dest any) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	node, ok := r.flows[id]
	if !ok {
		return fmt.Errorf("flow configuration not found: %s", id)
	}
	return node.Decode(dest)
}

func (r *Registry) DecodeCoreConfig(id string, dest any) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	node, ok := r.core[id]
	if !ok {
		return fmt.Errorf("core configuration not found: %s", id)
	}
	return node.Decode(dest)
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
