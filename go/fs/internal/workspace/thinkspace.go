package workspace

import (
	"context"
	"time"

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

	TurnTimeoutSeconds   int `yaml:"turn_timeout_seconds"`
	AgentTimeoutSeconds  int `yaml:"agent_timeout_seconds"`
	VerifyTimeoutSeconds int `yaml:"verify_timeout_seconds"`
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

	if c.TurnTimeoutSeconds == 0 {
		c.TurnTimeoutSeconds = 300 // 5 minutes default for UI responsiveness
	}
	if c.AgentTimeoutSeconds == 0 {
		c.AgentTimeoutSeconds = 60 // 1 minute for a sub-agent generation
	}
	if c.VerifyTimeoutSeconds == 0 {
		c.VerifyTimeoutSeconds = 15 // 15 seconds to catch infinite test loops
	}
}

type ThinkSpace interface {
	Name() string
	SystemPrompt() string
	SubAgentSystemPrompt() string
	Model(category ModelCategory) string
	Tools() []*genai.Tool
	Verify(ctx context.Context, dir string) error

	TurnTimeout() time.Duration
	AgentTimeout() time.Duration
	VerifyTimeout() time.Duration
}
