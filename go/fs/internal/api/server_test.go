package api_test

import (
	"context"
	"iter"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/api"
	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type mockModelClient struct{}

func (m *mockModelClient) GenerateContentStream(ctx context.Context, model string, bundle cognition.ThinkBundle, tools []llm.ToolDef) iter.Seq2[llm.StreamChunk, error] {
	return func(yield func(llm.StreamChunk, error) bool) {}
}

type dummyFlow struct{}

func (f *dummyFlow) Name() string  { return "dummy" }
func (f *dummyFlow) MaxSteps() int { return 1 }
func (f *dummyFlow) Next(state flows.FlowState) flows.FlowDirective {
	return flows.FlowDirective{Action: flows.ActionFinish}
}

func setupTestServer(t *testing.T) (*http.ServeMux, string, *workspace.ServiceManager) {
	tempDir := t.TempDir()
	configsDir := filepath.Join(tempDir, "configs")
	baseRoot := filepath.Join(tempDir, "workspace")

	if err := os.MkdirAll(configsDir, 0755); err != nil {
		t.Fatalf("failed to create configs dir: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(baseRoot, "golang"), 0755); err != nil {
		t.Fatalf("failed to create golang workspace dir: %v", err)
	}

	yamlContent := `golang:
  type: space
  name: test-domain
  system_prompt: "You are a test assistant."
  roles:
    manager: "manager rules"
    worker: "worker rules"
  models:
    manager: "gemini-test-manager"
    worker: "gemini-test-worker"

fanout:
  type: flow
  name: FanOut Flow
  retry_prompt: "retry"
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

	factory := func(repoRoot string) workspace.ChatEngine { return nil }

	manager := workspace.NewServiceManager(logger, baseRoot, factory, nil)

	mockClient := &mockModelClient{}
	dummyFlowInstance := &dummyFlow{}
	digestManager := session.NewDigestManager(logger, mockClient, session.DigestConfig{})

	mux := http.NewServeMux()

	appConfig := workspace.AppConfig{SupportedDomains: []string{"golang"}}
	mgmtAPI := api.NewManagementAPI(registry, manager, appConfig)
	mgmtAPI.RegisterHandlers(mux)

	wsServer := api.NewServer(logger, registry, manager, dummyFlowInstance, digestManager, mockClient)
	wsServer.RegisterHandlers(mux)

	return mux, "golang", manager
}

func TestServer_LegacyRoutes(t *testing.T) {
	mux, _, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/spaces", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", rr.Code)
	}
}
