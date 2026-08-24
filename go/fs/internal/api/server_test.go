package api_test

import (
	"context"
	"encoding/json"
	"iter"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/genai"

	"github.com/tinywideclouds.com/thinkspace/internal/api"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/gitfs"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// mockModelClient implements llm.ModelClient for testing
type mockModelClient struct{}

func (m *mockModelClient) GenerateContentStream(ctx context.Context, model string, history []*genai.Content, genConfig *genai.GenerateContentConfig) iter.Seq2[*genai.GenerateContentResponse, error] {
	return func(yield func(*genai.GenerateContentResponse, error) bool) {}
}

// dummyFlow implements workspace.Flow for testing
type dummyFlow struct{}

func (f *dummyFlow) Name() string { return "dummy" }
func (f *dummyFlow) Execute(ctx context.Context, svc *workspace.Service, thread *workspace.Thread, space workspace.ThinkSpace, args map[string]any, executor workspace.SubAgentExecutor, tokenChan chan<- workspace.AgentToken) (*workspace.FlowResult, error) {
	return &workspace.FlowResult{}, nil
}

func setupTestServer(t *testing.T) (*api.Server, string) {
	tempDir := t.TempDir()
	configsDir := filepath.Join(tempDir, "configs")
	workspaceRoot := filepath.Join(tempDir, "workspace")

	if err := os.MkdirAll(configsDir, 0755); err != nil {
		t.Fatalf("failed to create configs dir: %v", err)
	}

	yamlContent := `name: test-domain
system_prompt: "You are a test assistant."
models:
  manager: "gemini-test-manager"
  worker: "gemini-test-worker"
`
	configPath := filepath.Join(configsDir, "golang.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write dummy config: %v", err)
	}

	registry := config.NewRegistry()
	if err := registry.LoadDirectory(configsDir); err != nil {
		t.Fatalf("failed to load configs: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	stateEngine := gitfs.NewGoExecEngine()
	svc := workspace.NewService(logger, stateEngine, workspaceRoot)
	mockClient := &mockModelClient{}
	llmMgr := llm.NewManager(mockClient)
	flow := &dummyFlow{}

	srv := api.NewServer(logger, registry, svc, llmMgr, flow, mockClient)

	return srv, "golang"
}

func TestServer_HandleGetSpaces(t *testing.T) {
	srv, expectedSpaceID := setupTestServer(t)
	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/spaces", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	var response map[string]workspace.ThinkSpaceConfig
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	spaceConfig, ok := response[expectedSpaceID]
	if !ok {
		t.Fatalf("expected space %s not found in response", expectedSpaceID)
	}

	if spaceConfig.Name != "test-domain" {
		t.Errorf("expected space name 'test-domain', got '%s'", spaceConfig.Name)
	}
	if spaceConfig.SystemPrompt != "You are a test assistant." {
		t.Errorf("expected system prompt 'You are a test assistant.', got '%s'", spaceConfig.SystemPrompt)
	}
}
