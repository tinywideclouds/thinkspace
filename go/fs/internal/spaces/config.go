package spaces

import (
	"strings"
	"time"
)

// ThinkSpaceConfig represents the root YAML configuration for a domain space.
type ThinkSpaceConfig struct {
	Type         string         `yaml:"type"`
	Name         string         `yaml:"name"`
	SystemPrompt string         `yaml:"system_prompt"`
	Timeouts     TimeoutsConfig `yaml:"timeouts"`
	Roles        RolesConfig    `yaml:"roles"`
	Memory       MemoryConfig   `yaml:"memory"`
}

type TimeoutsConfig struct {
	TurnSeconds   int `yaml:"turn_seconds"`
	AgentSeconds  int `yaml:"agent_seconds"`
	VerifySeconds int `yaml:"verify_seconds"`
}

type RolesConfig struct {
	Manager ManagerConfig `yaml:"manager"`
	Worker  WorkerConfig  `yaml:"worker"`
}

type ManagerConfig struct {
	Model        string      `yaml:"model"`
	SystemPrompt string      `yaml:"system_prompt"`
	Tools        ToolsConfig `yaml:"tools"`
}

type WorkerConfig struct {
	Model               string `yaml:"model"`
	MaxTokens           int    `yaml:"max_tokens"`
	SystemPrompt        string `yaml:"system_prompt"`
	PatcherInstructions string `yaml:"patcher_instructions"`
	RetryPrompt         string `yaml:"retry_prompt"`
}

type ToolsConfig struct {
	ProposeChange ToolProposeChangeConfig `yaml:"propose_change"`
	QueryLens     ToolQueryLensConfig     `yaml:"query_lens"`
	CreateDigest  ToolCreateDigestConfig  `yaml:"create_digest"`
}

type ToolCreateDigestConfig struct {
	Description            string `yaml:"description"`
	TopicNameDescription   string `yaml:"topic_name_description"`
	TagsDescription        string `yaml:"tags_description"`
	InstructionDescription string `yaml:"instruction_description"`
}

type ToolProposeChangeConfig struct {
	Description                  string `yaml:"description"`
	AssignedTagsDescription      string `yaml:"assigned_tags_description"`
	AgentCountDescription        string `yaml:"agent_count_description"`
	AgentInstructionsDescription string `yaml:"agent_instructions_description"`
	ContextDigestDescription     string `yaml:"context_digest_description"`
	InstructionDescription       string `yaml:"instruction_description"`
	TargetFilesDescription       string `yaml:"target_files_description"`
}

type ToolQueryLensConfig struct {
	Description          string `yaml:"description"`
	TagDescription       string `yaml:"tag_description"`
	ReasoningDescription string `yaml:"reasoning_description"`
}

// Tuning parameters for the sliding window memory system
type MemoryConfig struct {
	TriggerThreshold int `yaml:"trigger_threshold"` // Soft check (e.g., 12)
	MaxVolatile      int `yaml:"max_volatile"`      // Hard limit (e.g., 20)
	ActiveBuffer     int `yaml:"active_buffer"`     // Turns to always keep verbatim (e.g., 6)
}

// ApplyDefaults sets reasonable timeouts if they are missing from the configuration.
func (c *ThinkSpaceConfig) ApplyDefaults() {
	if c.Timeouts.TurnSeconds == 0 {
		c.Timeouts.TurnSeconds = 300
	}
	if c.Timeouts.AgentSeconds == 0 {
		c.Timeouts.AgentSeconds = 60
	}
	if c.Timeouts.VerifySeconds == 0 {
		c.Timeouts.VerifySeconds = 15
	}
	if c.Memory.TriggerThreshold == 0 {
		c.Memory.TriggerThreshold = 12
	}
	if c.Memory.MaxVolatile == 0 {
		c.Memory.MaxVolatile = 20
	}
	if c.Memory.ActiveBuffer == 0 {
		c.Memory.ActiveBuffer = 6
	}
}

// Helper methods to compose the root domain rules with the specific role instructions.
func (c ThinkSpaceConfig) ManagerSystemPrompt() string {
	var parts []string
	if c.SystemPrompt != "" {
		parts = append(parts, c.SystemPrompt)
	}
	if c.Roles.Manager.SystemPrompt != "" {
		parts = append(parts, c.Roles.Manager.SystemPrompt)
	}
	return strings.Join(parts, "\n\n")
}

func (c ThinkSpaceConfig) WorkerSystemPrompt() string {
	var parts []string
	if c.SystemPrompt != "" {
		parts = append(parts, c.SystemPrompt)
	}
	if c.Roles.Worker.SystemPrompt != "" {
		parts = append(parts, c.Roles.Worker.SystemPrompt)
	}
	return strings.Join(parts, "\n\n")
}

func (c ThinkSpaceConfig) TurnTimeout() time.Duration {
	return time.Duration(c.Timeouts.TurnSeconds) * time.Second
}

func (c ThinkSpaceConfig) AgentTimeout() time.Duration {
	return time.Duration(c.Timeouts.AgentSeconds) * time.Second
}

func (c ThinkSpaceConfig) VerifyTimeout() time.Duration {
	return time.Duration(c.Timeouts.VerifySeconds) * time.Second
}
