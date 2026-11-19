package spaces_test

import (
	"testing"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"gopkg.in/yaml.v3"
)

func TestThinkSpaceConfig_ApplyDefaults(t *testing.T) {
	configuration := spaces.ThinkStateConfig{}

	// Apply defaults to an empty configuration
	configuration.ApplyDefaults()

	if configuration.Timeouts.TurnSeconds != 300 {
		t.Errorf("Expected TurnTimeoutSeconds default to be 300, got %d", configuration.Timeouts.TurnSeconds)
	}
	if configuration.Timeouts.AgentSeconds != 60 {
		t.Errorf("Expected AgentTimeoutSeconds default to be 60, got %d", configuration.Timeouts.AgentSeconds)
	}
	if configuration.Timeouts.VerifySeconds != 15 {
		t.Errorf("Expected VerifyTimeoutSeconds default to be 15, got %d", configuration.Timeouts.VerifySeconds)
	}
}

func TestThinkSpaceConfig_DoNotOverrideExplicitValues(t *testing.T) {
	configuration := spaces.ThinkStateConfig{
		Timeouts: spaces.TimeoutsConfig{
			TurnSeconds:   500,
			AgentSeconds:  120,
			VerifySeconds: 45,
		},
	}

	// Apply defaults should not overwrite existing values
	configuration.ApplyDefaults()

	if configuration.Timeouts.TurnSeconds != 500 {
		t.Errorf("Expected TurnTimeoutSeconds to remain 500, got %d", configuration.Timeouts.TurnSeconds)
	}
	if configuration.Timeouts.AgentSeconds != 120 {
		t.Errorf("Expected AgentTimeoutSeconds to remain 120, got %d", configuration.Timeouts.AgentSeconds)
	}
	if configuration.Timeouts.VerifySeconds != 45 {
		t.Errorf("Expected VerifyTimeoutSeconds to remain 45, got %d", configuration.Timeouts.VerifySeconds)
	}
}

func TestThinkSpaceConfig_SystemPrompts(t *testing.T) {
	configuration := spaces.ThinkStateConfig{
		SystemPrompt: "Base rules.",
		Roles: spaces.RolesConfig{
			Manager: spaces.ManagerConfig{
				SystemPrompt: "You are the manager.",
			},
			Worker: spaces.WorkerConfig{
				SystemPrompt: "You are the worker.",
			},
		},
	}

	managerPrompt := configuration.ManagerSystemPrompt()
	expectedManager := "Base rules.\n\nYou are the manager."
	if managerPrompt != expectedManager {
		t.Errorf("Expected manager prompt %q, got %q", expectedManager, managerPrompt)
	}

	workerPrompt := configuration.WorkerSystemPrompt()
	expectedWorker := "Base rules.\n\nYou are the worker."
	if workerPrompt != expectedWorker {
		t.Errorf("Expected worker prompt %q, got %q", expectedWorker, workerPrompt)
	}
}

func TestThinkSpaceConfig_Timeouts(t *testing.T) {
	configuration := spaces.ThinkStateConfig{
		Timeouts: spaces.TimeoutsConfig{
			TurnSeconds:   10,
			AgentSeconds:  20,
			VerifySeconds: 30,
		},
	}

	if configuration.TurnTimeout() != 10*time.Second {
		t.Errorf("Expected 10s TurnTimeout, got %v", configuration.TurnTimeout())
	}
	if configuration.AgentTimeout() != 20*time.Second {
		t.Errorf("Expected 20s AgentTimeout, got %v", configuration.AgentTimeout())
	}
	if configuration.VerifyTimeout() != 30*time.Second {
		t.Errorf("Expected 30s VerifyTimeout, got %v", configuration.VerifyTimeout())
	}
}

func TestThinkSpaceConfig_DomainAgnosticOptionsAndToolDescriptions(t *testing.T) {
	rawYAML := `
name: literary-critique
options:
  citation_style: chicago
  strict_meter_check: true
roles:
  manager:
    tools:
      propose_change:
        description: "Delegate prose drafting"
        intention_description: "Explain why drafting agents are being spawned"
`
	var cfg spaces.ThinkStateConfig
	if err := yaml.Unmarshal([]byte(rawYAML), &cfg); err != nil {
		t.Fatalf("failed to unmarshal ThinkStateConfig: %v", err)
	}

	if cfg.Options["citation_style"] != "chicago" {
		t.Errorf("expected citation_style 'chicago', got %v", cfg.Options["citation_style"])
	}
	if cfg.Options["strict_meter_check"] != true {
		t.Errorf("expected strict_meter_check true, got %v", cfg.Options["strict_meter_check"])
	}
	if cfg.Roles.Manager.Tools.ProposeChange.IntentionDescription != "Explain why drafting agents are being spawned" {
		t.Errorf("unexpected IntentionDescription: %q", cfg.Roles.Manager.Tools.ProposeChange.IntentionDescription)
	}
}
