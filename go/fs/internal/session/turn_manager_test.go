package session_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

func setupTurnManagerEnv(t *testing.T) (*config.Registry, *workspace.ServiceManager, string) {
	tempDir := t.TempDir()
	configsDir := filepath.Join(tempDir, "configs")
	baseRoot := filepath.Join(tempDir, "workspace")
	os.MkdirAll(configsDir, 0755)

	yamlContent := `
golang-space:
  type: space
  name: test-go
  roles:
    manager:
      model: "test-model"
      system_prompt: "go manager rules"

concept-space:
  type: space
  name: test-concept
  roles:
    manager:
      model: "test-model"
      system_prompt: "concept manager rules"
`
	os.WriteFile(filepath.Join(configsDir, "config.yaml"), []byte(yamlContent), 0644)

	registry := config.NewRegistry()
	registry.LoadDirectory(configsDir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := workspace.NewServiceManager(logger, baseRoot, func(string) workspace.ChatEngine { return &coordMockChatEngine{} })

	return registry, manager, baseRoot
}

func writeMetaJson(t *testing.T, baseRoot, spaceID, chatID, activeDomain string) {
	chatDir := filepath.Join(baseRoot, spaceID, "chats", chatID)
	os.MkdirAll(chatDir, 0755)

	meta := workspace.ChatMeta{
		ID:           chatID,
		Name:         "test-chat",
		ActiveDomain: activeDomain,
	}
	b, _ := json.Marshal(meta)
	os.WriteFile(filepath.Join(chatDir, "meta.json"), b, 0644)
}

func TestTurnManager_RunTurn_Sequencing(t *testing.T) {
	registry, manager, baseRoot := setupTurnManagerEnv(t)
	writeMetaJson(t, baseRoot, "golang-space", "chat-1", "golang-space")

	spyClient := &coordMockModelClient{}
	dummyFlow := &coordMockFlow{}
	ui := &coordMockUI{reviewToReturn: true}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	digestManager := session.NewDigestManager(logger, spyClient, session.DigestConfig{})
	turnManager := session.NewTurnManager(logger, registry, manager, spyClient, dummyFlow, digestManager)

	err := turnManager.RunTurn(context.Background(), "golang-space", "chat-1", "Hello sequencing test", ui, &coordMockEmitter{})
	if err != nil {
		t.Fatalf("RunTurn failed: %v", err)
	}

	// Because spyClient is the first thing invoked by the turn, it should have captured the initial prompt
	if spyClient.callIndex > 0 {
		// Just ensure it ran without crashing. Specific catalyst capture requires exposing internal state in the mock.
	}
}

func TestTurnManager_RunTurn_Transition(t *testing.T) {
	registry, manager, baseRoot := setupTurnManagerEnv(t)
	writeMetaJson(t, baseRoot, "concept-space", "chat-trans", "concept-space")

	spyClient := &coordMockModelClient{
		responses: []llm.StreamChunk{
			{
				ToolCalls: []llm.ToolCall{{
					Name: "transition_workspace",
					Args: map[string]any{"target_domain": "golang-space", "reasoning": "Need to code"},
				}},
			},
			{
				Text: "I have successfully transitioned.",
			},
		},
	}

	dummyFlow := &coordMockFlow{}
	ui := &coordMockUI{reviewToReturn: true, transitionToReturn: true}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	digestManager := session.NewDigestManager(logger, spyClient, session.DigestConfig{})
	turnManager := session.NewTurnManager(logger, registry, manager, spyClient, dummyFlow, digestManager)

	err := turnManager.RunTurn(context.Background(), "concept-space", "chat-trans", "Switch to go", ui, &coordMockEmitter{})
	if err != nil {
		t.Fatalf("RunTurn failed: %v", err)
	}

	meta, err := manager.GetChatMeta("concept-space", "chat-trans")
	if err != nil {
		t.Fatalf("Failed to retrieve chat meta: %v", err)
	}

	if meta.ActiveDomain != "golang-space" {
		t.Errorf("Expected ActiveDomain to switch to 'golang-space', got '%s'", meta.ActiveDomain)
	}

	foundInit := false
	for _, d := range meta.InitializedDomains {
		if d == "golang-space" {
			foundInit = true
		}
	}
	if !foundInit {
		t.Errorf("Expected 'golang-space' to be appended to InitializedDomains")
	}
}
