package config

import (
	"fmt"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"

	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
)

type ConfigHeader struct {
	Type string `yaml:"type"` // "thinkstate", "flow", or "core"
}

// FanOutFlowConfig defines the typed configuration and prompt templates for the parallel fan-out flow.
type FanOutFlowConfig struct {
	Version               int    `yaml:"version"`
	Type                  string `yaml:"type"`
	Name                  string `yaml:"name"`
	Description           string `yaml:"description"`
	RetryPrompt           string `yaml:"retry_prompt"`
	ReviewPrompt          string `yaml:"review_prompt"`
	RefinePrompt          string `yaml:"refine_prompt"`
	SynthesizePrompt      string `yaml:"synthesize_prompt"`
	SynthesisReviewPrompt string `yaml:"synthesis_review_prompt"`
}

// Validate ensures all required flow prompts are present and parse cleanly as Go templates.
func (c FanOutFlowConfig) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("fanout flow config is missing required field 'name'")
	}

	requiredPrompts := []struct {
		field string
		value string
	}{
		{field: "retry_prompt", value: c.RetryPrompt},
		{field: "review_prompt", value: c.ReviewPrompt},
		{field: "refine_prompt", value: c.RefinePrompt},
		{field: "synthesize_prompt", value: c.SynthesizePrompt},
		{field: "synthesis_review_prompt", value: c.SynthesisReviewPrompt},
	}

	for _, p := range requiredPrompts {
		if strings.TrimSpace(p.value) == "" {
			return fmt.Errorf("fanout flow config is missing required prompt '%s'", p.field)
		}
		if _, err := template.New(p.field).Parse(p.value); err != nil {
			return fmt.Errorf("fanout flow config has invalid template in '%s': %w", p.field, err)
		}
	}

	return nil
}

type ParsedConfig struct {
	ThinkStates map[string]spaces.ThinkStateConfig
	Flows       map[string]yaml.Node
	Core        map[string]yaml.Node
}

func ParseConfigBytes(data []byte) (*ParsedConfig, error) {
	var rawMap map[string]yaml.Node
	if err := yaml.Unmarshal(data, &rawMap); err != nil {
		return nil, fmt.Errorf("invalid YAML structure: %w", err)
	}

	result := &ParsedConfig{
		ThinkStates: make(map[string]spaces.ThinkStateConfig),
		Flows:       make(map[string]yaml.Node),
		Core:        make(map[string]yaml.Node),
	}

	for id, node := range rawMap {
		var header ConfigHeader
		if err := node.Decode(&header); err != nil {
			return nil, fmt.Errorf("invalid config block '%s': %w", id, err)
		}

		switch header.Type {
		case "flow":
			if id == "fanout" {
				var fanoutCfg FanOutFlowConfig
				if err := node.Decode(&fanoutCfg); err != nil {
					return nil, fmt.Errorf("failed to parse flow '%s': %w", id, err)
				}
				if err := fanoutCfg.Validate(); err != nil {
					return nil, err
				}
			}
			result.Flows[id] = node

		case "core":
			result.Core[id] = node

		case "thinkstate":
			var cfg spaces.ThinkStateConfig
			if err := node.Decode(&cfg); err != nil {
				return nil, fmt.Errorf("failed to parse space '%s': %w", id, err)
			}
			cfg.ApplyDefaults()
			result.ThinkStates[id] = cfg

		default:
			return nil, fmt.Errorf("unknown config type '%s' for '%s'", header.Type, id)
		}
	}

	return result, nil
}
