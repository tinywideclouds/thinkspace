package workspace

import (
	"context"

	"google.golang.org/genai"
)

type ModelCategory string

const (
	ModelCategoryManager ModelCategory = "manager"
	ModelCategoryWorker  ModelCategory = "worker"
)

type ThinkSpaceConfig struct {
	Name                         string                   `yaml:"name"`
	Models                       map[ModelCategory]string `yaml:"models"`
	SystemPrompt                 string                   `yaml:"system_prompt"`
	SubAgentSystemPrompt         string                   `yaml:"sub_agent_system_prompt"`
	ToolDescription              string                   `yaml:"tool_description"`
	AgentCountDescription        string                   `yaml:"agent_count_description"`
	AgentInstructionsDescription string                   `yaml:"agent_instructions_description"`
}

// ApplyDefaults ensures required configuration fields have safe fallbacks.
func (c *ThinkSpaceConfig) ApplyDefaults() {
	if c.Models == nil {
		c.Models = make(map[ModelCategory]string)
	}

	// Enforce global defaults so domain implementations stay clean
	if c.Models[ModelCategoryManager] == "" {
		c.Models[ModelCategoryManager] = "gemini-3.5-pro"
	}
	if c.Models[ModelCategoryWorker] == "" {
		c.Models[ModelCategoryWorker] = "gemini-3.6-flash"
	}
}

type ThinkSpace interface {
	Name() string
	SystemPrompt() string
	SubAgentSystemPrompt() string
	Model(category ModelCategory) string
	Tools() []*genai.Tool
	Verify(ctx context.Context, dir string) error
}
