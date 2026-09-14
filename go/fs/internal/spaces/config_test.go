package spaces_test

import (
	"testing"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
)

func TestThinkSpaceConfig_ApplyDefaults(t *testing.T) {
	configuration := spaces.ThinkSpaceConfig{}

	// Apply defaults to an empty configuration
	configuration.ApplyDefaults()

	if configuration.TurnTimeoutSeconds != 300 {
		t.Errorf("Expected TurnTimeoutSeconds default to be 300, got %d", configuration.TurnTimeoutSeconds)
	}
	if configuration.AgentTimeoutSeconds != 60 {
		t.Errorf("Expected AgentTimeoutSeconds default to be 60, got %d", configuration.AgentTimeoutSeconds)
	}
	if configuration.VerifyTimeoutSeconds != 15 {
		t.Errorf("Expected VerifyTimeoutSeconds default to be 15, got %d", configuration.VerifyTimeoutSeconds)
	}
}

func TestThinkSpaceConfig_DoNotOverrideExplicitValues(t *testing.T) {
	configuration := spaces.ThinkSpaceConfig{
		TurnTimeoutSeconds:   500,
		AgentTimeoutSeconds:  120,
		VerifyTimeoutSeconds: 45,
	}

	// Apply defaults should not overwrite existing values
	configuration.ApplyDefaults()

	if configuration.TurnTimeoutSeconds != 500 {
		t.Errorf("Expected TurnTimeoutSeconds to remain 500, got %d", configuration.TurnTimeoutSeconds)
	}
	if configuration.AgentTimeoutSeconds != 120 {
		t.Errorf("Expected AgentTimeoutSeconds to remain 120, got %d", configuration.AgentTimeoutSeconds)
	}
	if configuration.VerifyTimeoutSeconds != 45 {
		t.Errorf("Expected VerifyTimeoutSeconds to remain 45, got %d", configuration.VerifyTimeoutSeconds)
	}
}

func TestThinkSpaceConfig_SystemPrompts(t *testing.T) {
	configuration := spaces.ThinkSpaceConfig{
		SystemPrompt: "Base system instructions.",
		Roles: spaces.ThinkSpaceRoles{
			Manager: "You are the manager.",
			Worker:  "You are the worker.",
		},
	}

	managerPrompt := configuration.ManagerSystemPrompt()
	expectedManager := "Base system instructions.\n\nYou are the manager."
	if managerPrompt != expectedManager {
		t.Errorf("Expected manager prompt %q, got %q", expectedManager, managerPrompt)
	}

	workerPrompt := configuration.WorkerSystemPrompt()
	expectedWorker := "Base system instructions.\n\nYou are the worker."
	if workerPrompt != expectedWorker {
		t.Errorf("Expected worker prompt %q, got %q", expectedWorker, workerPrompt)
	}
}

func TestThinkSpaceConfig_Timeouts(t *testing.T) {
	configuration := spaces.ThinkSpaceConfig{
		TurnTimeoutSeconds:   10,
		AgentTimeoutSeconds:  20,
		VerifyTimeoutSeconds: 30,
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
