package config

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
)

type ConfigHeader struct {
	Type string `yaml:"type"` // "space", "flow", or "core"
}

type ParsedConfig struct {
	Spaces map[string]spaces.ThinkStateConfig
	Flows  map[string]yaml.Node
	Core   map[string]yaml.Node
}

func ParseConfigBytes(data []byte) (*ParsedConfig, error) {
	var rawMap map[string]yaml.Node
	if err := yaml.Unmarshal(data, &rawMap); err != nil {
		return nil, fmt.Errorf("invalid YAML structure: %w", err)
	}

	result := &ParsedConfig{
		Spaces: make(map[string]spaces.ThinkStateConfig),
		Flows:  make(map[string]yaml.Node),
		Core:   make(map[string]yaml.Node),
	}

	for id, node := range rawMap {
		var header ConfigHeader
		if err := node.Decode(&header); err != nil {
			return nil, fmt.Errorf("invalid config block '%s': %w", id, err)
		}

		switch header.Type {
		case "flow":
			result.Flows[id] = node

		case "core":
			result.Core[id] = node

		case "space", "": // Default to space for backward compatibility
			var cfg spaces.ThinkStateConfig
			if err := node.Decode(&cfg); err != nil {
				return nil, fmt.Errorf("failed to parse space '%s': %w", id, err)
			}
			cfg.ApplyDefaults()
			result.Spaces[id] = cfg

		default:
			return nil, fmt.Errorf("unknown config type '%s' for '%s'", header.Type, id)
		}
	}

	return result, nil
}
