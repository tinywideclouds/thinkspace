package session_test

import (
	"context"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type spyModelClient struct {
	capturedBundle cognition.ThinkBundle
}

func (m *spyModelClient) GenerateContentStream(ctx context.Context, model string, bundle cognition.ThinkBundle, tools []llm.ToolDef) iter.Seq2[llm.StreamChunk, error] {
	m.capturedBundle = bundle
	return func(yield func(llm.StreamChunk, error) bool) {}
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
    manager:
      model: "test-model"
      system_prompt: "manager rules"
    worker:
      model: "test-worker-model"
      system_prompt: ""

fanout:
  type: flow
  name: test-flow
`
	os.WriteFile(filepath.Join(configsDir, "config.yaml"), []byte(yamlContent), 0644)

	registry := config.NewRegistry()
	registry.LoadDirectory(configsDir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := workspace.NewServiceManager(logger, baseRoot, func(string) workspace.ChatEngine { return &stubEngine{} })

	manager.UpdateSpaceState("golang-space", func(s *workspace.SpaceState) { s.Domain = "golang-space" })

	spyClient := &spyModelClient{}
	dummyFlow := &mockFlow{}
	ui := &mockUserInterface{reviewToReturn: true}

	turnManager := session.NewTurnManager(logger, registry, manager, spyClient, dummyFlow)

	err := turnManager.ExecuteTurn(context.Background(), "golang-space", "chat-1", "Hello sequencing test", ui, &mockEmitter{})
	if err != nil {
		t.Fatalf("ExecuteTurn failed: %v", err)
	}

	if spyClient.capturedBundle.Trajectory.Catalyst.Content != "Hello sequencing test" {
		t.Errorf("Expected catalyst to be the prompt, got: %s", spyClient.capturedBundle.Trajectory.Catalyst.Content)
	}

	if spyClient.capturedBundle.Rules.Persona != "manager rules" {
		t.Errorf("Expected rules to contain manager rules, got: %s", spyClient.capturedBundle.Rules.Persona)
	}

	if _, ok := spyClient.capturedBundle.Mapbook.Maps["Structural"]; !ok {
		t.Errorf("Expected Mapbook to contain Structural layer")
	}
}
