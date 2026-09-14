package spaces

import (
	"time"
)

// ThinkSpaceConfig represents the raw YAML configuration for a domain space.
type ThinkSpaceConfig struct {
	Type                         string                   `yaml:"type"`
	Name                         string                   `yaml:"name"`
	SystemPrompt                 string                   `yaml:"system_prompt"`
	Roles                        ThinkSpaceRoles          `yaml:"roles"`
	Models                       map[ModelCategory]string `yaml:"models"`
	TurnTimeoutSeconds           int                      `yaml:"turn_timeout_seconds"`
	AgentTimeoutSeconds          int                      `yaml:"agent_timeout_seconds"`
	VerifyTimeoutSeconds         int                      `yaml:"verify_timeout_seconds"`
	MaxWorkerTokens              int                      `yaml:"max_worker_tokens"`
	ToolDescription              string                   `yaml:"tool_description"`
	AgentCountDescription        string                   `yaml:"agent_count_description"`
	AssignedTagsDescription      string                   `yaml:"assigned_tags_description"`
	AgentInstructionsDescription string                   `yaml:"agent_instructions_description"`
	ContextDigestDescription     string                   `yaml:"context_digest_description"`
	InstructionDescription       string                   `yaml:"instruction_description"`
	TargetFilesDescription       string                   `yaml:"target_files_description"`
	WorkerRetryPrompt            string                   `yaml:"worker_retry_prompt"`
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

// Helper methods bound directly to the config struct.
func (c ThinkSpaceConfig) ManagerSystemPrompt() string {
	return c.SystemPrompt + "\n\n" + c.Roles.Manager
}

func (c ThinkSpaceConfig) WorkerSystemPrompt() string {
	return c.SystemPrompt + "\n\n" + c.Roles.Worker
}

func (c ThinkSpaceConfig) TurnTimeout() time.Duration {
	return time.Duration(c.TurnTimeoutSeconds) * time.Second
}

func (c ThinkSpaceConfig) AgentTimeout() time.Duration {
	return time.Duration(c.AgentTimeoutSeconds) * time.Second
}

func (c ThinkSpaceConfig) VerifyTimeout() time.Duration {
	return time.Duration(c.VerifyTimeoutSeconds) * time.Second
}
