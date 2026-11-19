package spaces

import (
	"strings"
	"time"
)

// ThinkStateConfig represents the root YAML configuration for a domain space.
type ThinkStateConfig struct {
	Type         string          `yaml:"type"`
	Name         string          `yaml:"name"`
	SystemPrompt string          `yaml:"system_prompt"`
	Timeouts     TimeoutsConfig  `yaml:"timeouts"`
	Roles        RolesConfig     `yaml:"roles"`
	Memory       MemoryConfig    `yaml:"memory"`
	Options      map[string]any  `yaml:"options,omitempty"`
	Toolchain    ToolchainConfig `yaml:"toolchain"`
}

type ToolchainConfig struct {
	PreVerify []string `yaml:"pre_verify"`
	Verify    []string `yaml:"verify"`
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
	IsStickyDescription    string `yaml:"is_sticky_description"`
}

type ToolProposeChangeConfig struct {
	Description              string `yaml:"description"`
	IntentionDescription     string `yaml:"intention_description"`
	AssignedTagsDescription  string `yaml:"assigned_tags_description"`
	AgentCountDescription    string `yaml:"agent_count_description"`
	ContextDigestDescription string `yaml:"context_digest_description"`
	InstructionDescription   string `yaml:"instruction_description"`
	TargetFilesDescription   string `yaml:"target_files_description"`
}

type ToolQueryLensConfig struct {
	Description          string `yaml:"description"`
	TagDescription       string `yaml:"tag_description"`
	ReasoningDescription string `yaml:"reasoning_description"`
}

type MemoryConfig struct {
	TriggerThreshold int `yaml:"trigger_threshold"`
	MaxVolatile      int `yaml:"max_volatile"`
	ActiveBuffer     int `yaml:"active_buffer"`
	FocusDepth       int `yaml:"focus_depth"` // Controls the semantic aperture
}

func (c *ThinkStateConfig) ApplyDefaults() {
	if c.Timeouts.TurnSeconds <= 0 {
		c.Timeouts.TurnSeconds = 300
	}
	if c.Timeouts.AgentSeconds <= 0 {
		c.Timeouts.AgentSeconds = 60
	}
	if c.Timeouts.VerifySeconds <= 0 {
		c.Timeouts.VerifySeconds = 15
	}
	if c.Memory.TriggerThreshold <= 0 {
		c.Memory.TriggerThreshold = 12
	}
	if c.Memory.MaxVolatile <= 0 {
		c.Memory.MaxVolatile = 20
	}
	if c.Memory.ActiveBuffer <= 0 {
		c.Memory.ActiveBuffer = 6
	}
	if c.Memory.FocusDepth <= 0 {
		c.Memory.FocusDepth = 3
	}
}

func (c ThinkStateConfig) ManagerSystemPrompt() string {
	var parts []string
	if c.SystemPrompt != "" {
		parts = append(parts, c.SystemPrompt)
	}
	if c.Roles.Manager.SystemPrompt != "" {
		parts = append(parts, c.Roles.Manager.SystemPrompt)
	}
	return strings.Join(parts, "\n\n")
}

func (c ThinkStateConfig) WorkerSystemPrompt() string {
	var parts []string
	if c.SystemPrompt != "" {
		parts = append(parts, c.SystemPrompt)
	}
	if c.Roles.Worker.SystemPrompt != "" {
		parts = append(parts, c.Roles.Worker.SystemPrompt)
	}
	return strings.Join(parts, "\n\n")
}

func (c ThinkStateConfig) TurnTimeout() time.Duration {
	return time.Duration(c.Timeouts.TurnSeconds) * time.Second
}

func (c ThinkStateConfig) AgentTimeout() time.Duration {
	return time.Duration(c.Timeouts.AgentSeconds) * time.Second
}

func (c ThinkStateConfig) VerifyTimeout() time.Duration {
	return time.Duration(c.Timeouts.VerifySeconds) * time.Second
}
