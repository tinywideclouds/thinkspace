package session_test

import (
	"context"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/genai"

	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type spyModelClient struct {
	capturedHistory []*genai.Content
	capturedConfig  *genai.GenerateContentConfig
}

func (m *spyModelClient) GenerateContentStream(ctx context.Context, model string, history []*genai.Content, config *genai.GenerateContentConfig) iter.Seq2[*genai.GenerateContentResponse, error] {
	m.capturedHistory = history
	m.capturedConfig = config
	return func(yield func(*genai.GenerateContentResponse, error) bool) {}
}

type stubEngine struct{}

func (s *stubEngine) InitChat(ctx context.Context, chatID string) error { return nil }
func (s *stubEngine) Snapshot(ctx context.Context, chatID string, message string) (string, error) {
	return "", nil
}
func (s *stubEngine) SpawnCandidateSandbox(ctx context.Context, chatID string, candidateID string) (workspace.CandidateSandbox, error) {
	return nil, nil
}
func (s *stubEngine) PreviewCandidate(ctx context.Context, chatID string, candidateID string) error {
	return nil
}
func (s *stubEngine) Accept(ctx context.Context, chatID string, candidateID string, reason string) error {
	return nil
}
func (s *stubEngine) Reject(ctx context.Context, chatID string, candidateID string, reason string) error {
	return nil
}
func (s *stubEngine) ReadCandidateDiff(ctx context.Context, chatID, candidateID string) (string, error) {
	return "", nil
}

func TestTurnManager_Sequencing(t *testing.T) {
	tempDir := t.TempDir()
	configsDir := filepath.Join(tempDir, "configs")
	baseRoot := filepath.Join(tempDir, "workspace")
	os.MkdirAll(configsDir, 0755)
	os.MkdirAll(filepath.Join(baseRoot, "golang-space"), 0755)

	yamlContent := `
golang-space:
  type: space
  name: test
  roles:
    manager: ""
    worker: ""
  models:
    manager: "test-model"

fanout:
  type: flow
  name: test-flow
`
	os.WriteFile(filepath.Join(configsDir, "config.yaml"), []byte(yamlContent), 0644)

	registry := config.NewRegistry()
	registry.LoadDirectory(configsDir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := workspace.NewServiceManager(logger, baseRoot, func(string) workspace.ChatEngine { return &stubEngine{} })

	// Manually configure the space to avoid HTTP management API dependencies
	manager.UpdateSpaceState("golang-space", func(s *workspace.SpaceState) { s.Domain = "golang-space" })

	spyClient := &spyModelClient{}
	llmAdapter := llm.NewAdapter(spyClient)
	dummyFlow := &mockFlow{}
	ui := &mockUserInterface{}

	turnManager := session.NewTurnManager(logger, registry, manager, llmAdapter, dummyFlow, spyClient)

	// Execute a turn with a new prompt
	err := turnManager.ExecuteTurn(context.Background(), "golang-space", "chat-1", "Hello sequencing test", ui, &mockEmitter{})
	if err != nil {
		t.Fatalf("ExecuteTurn failed: %v", err)
	}

	// Assert the context assembly was strictly ordered *after* the ledger write
	if len(spyClient.capturedHistory) == 0 {
		t.Fatalf("Expected history to be populated, got 0. The ledger load sequence is fundamentally broken.")
	}

	lastText := spyClient.capturedHistory[len(spyClient.capturedHistory)-1].Parts[0].Text
	if lastText != "Hello sequencing test" {
		t.Errorf("Expected last history item to be the prompt, got: %s", lastText)
	}

	// Assert dynamic mapbook injection in the system prompt
	if spyClient.capturedConfig == nil || spyClient.capturedConfig.SystemInstruction == nil {
		t.Fatalf("Expected SDK configuration to contain SystemInstruction")
	}

	sysPrompt := spyClient.capturedConfig.SystemInstruction.Parts[0].Text
	if !strings.Contains(sysPrompt, "### THE MAPBOOK") {
		t.Errorf("Expected Mapbook to be injected into system prompt, got: %s", sysPrompt)
	}
}
