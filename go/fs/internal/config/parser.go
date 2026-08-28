package config

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace/flows"
)

// ConfigHeader allows us to peek at the YAML type before full unmarshaling.
type ConfigHeader struct {
	Type string `yaml:"type"` // "space" or "flow"
}

// ParsedConfig holds the raw extracted configurations from a YAML file.
type ParsedConfig struct {
	Spaces map[string]workspace.ThinkSpaceConfig
	Flows  map[string]flows.FlowConfig
}

// ParseConfigBytes reads a YAML byte slice and decodes it into Spaces and Flows.
// It returns an error if the YAML is invalid, making it ideal for CLI validation tools.
func ParseConfigBytes(data []byte) (*ParsedConfig, error) {
	var rawMap map[string]yaml.Node
	if err := yaml.Unmarshal(data, &rawMap); err != nil {
		return nil, fmt.Errorf("invalid YAML structure: %w", err)
	}

	result := &ParsedConfig{
		Spaces: make(map[string]workspace.ThinkSpaceConfig),
		Flows:  make(map[string]flows.FlowConfig),
	}

	for id, node := range rawMap {
		var header ConfigHeader
		if err := node.Decode(&header); err != nil {
			return nil, fmt.Errorf("invalid config block '%s': %w", id, err)
		}

		switch header.Type {
		case "flow":
			var cfg flows.FlowConfig
			if err := node.Decode(&cfg); err != nil {
				return nil, fmt.Errorf("failed to parse flow '%s': %w", id, err)
			}
			result.Flows[id] = cfg

		case "space", "": // Default to space for backward compatibility
			var cfg workspace.ThinkSpaceConfig
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
