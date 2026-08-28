package workspace

import (
	"time"

	"google.golang.org/genai"
)

// ModelCategory defines the role of an LLM in the orchestration process.
type ModelCategory string

const (
	ModelCategoryManager ModelCategory = "manager"
	ModelCategoryWorker  ModelCategory = "worker"
)

// ThinkSpaceConfig represents the raw YAML configuration for a domain space.
type ThinkSpaceConfig struct {
	Type                         string                   `yaml:"type"`
	Name                         string                   `yaml:"name"`
	SystemPrompt                 string                   `yaml:"system_prompt"`
	SubAgentSystemPrompt         string                   `yaml:"sub_agent_system_prompt"`
	Models                       map[ModelCategory]string `yaml:"models"`
	TurnTimeoutSeconds           int                      `yaml:"turn_timeout_seconds"`
	AgentTimeoutSeconds          int                      `yaml:"agent_timeout_seconds"`
	VerifyTimeoutSeconds         int                      `yaml:"verify_timeout_seconds"`
	ToolDescription              string                   `yaml:"tool_description"`
	AgentCountDescription        string                   `yaml:"agent_count_description"`
	AgentInstructionsDescription string                   `yaml:"agent_instructions_description"`
	BaseAgentRules               string                   `yaml:"base_agent_rules"`
}

// ApplyDefaults sets reasonable timeouts if they are missing from the configuration.
func (c *ThinkSpaceConfig) ApplyDefaults() {
	if c.TurnTimeoutSeconds == 0 {
		c.TurnTimeoutSeconds = 300
	}
	if c.AgentTimeoutSeconds == 0 {
		c.AgentTimeoutSeconds = 60
	}
	if c.VerifyTimeoutSeconds == 0 {
		c.VerifyTimeoutSeconds = 15
	}
}

// ThinkSpace defines the contract for a language-specific or domain-specific environment.
type ThinkSpace interface {
	Name() string
	SystemPrompt() string
	SubAgentSystemPrompt() string
	Model(category ModelCategory) string
	TurnTimeout() time.Duration
	AgentTimeout() time.Duration
	VerifyTimeout() time.Duration
	Tools() []*genai.Tool

	// Verifier returns the domain-specific verification engine.
	Verifier() Verifier
}
