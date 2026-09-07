package api_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/api"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

func setupManagementAPI(t *testing.T) (*http.ServeMux, *workspace.ServiceManager) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	baseRoot := t.TempDir()

	factory := func(repoRoot string) workspace.ChatEngine { return nil }
	manager := workspace.NewServiceManager(logger, baseRoot, factory)

	registry := config.NewRegistry()
	appCfg := workspace.AppConfig{SupportedDomains: []string{"golang"}}

	mgmtAPI := api.NewManagementAPI(registry, manager, appCfg)
	mux := http.NewServeMux()
	mgmtAPI.RegisterHandlers(mux)

	return mux, manager
}

func TestManagementAPI_GetConfig(t *testing.T) {
	mux, _ := setupManagementAPI(t)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var cfg workspace.AppConfig
	if err := json.NewDecoder(rr.Body).Decode(&cfg); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(cfg.SupportedDomains) != 1 || cfg.SupportedDomains[0] != "golang" {
		t.Errorf("expected supported domains ['golang'], got '%v'", cfg.SupportedDomains)
	}
}

func TestManagementAPI_GetSpaces(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	baseRoot := t.TempDir()
	configsDir := filepath.Join(t.TempDir(), "configs")
	_ = os.MkdirAll(configsDir, 0755)

	// 1. Setup the Template Registry
	yamlContent := `golang:
  type: space
  name: "Golang Domain"
`
	_ = os.WriteFile(filepath.Join(configsDir, "golang.yaml"), []byte(yamlContent), 0644)
	registry := config.NewRegistry()
	_ = registry.LoadDirectory(configsDir)

	manager := workspace.NewServiceManager(logger, baseRoot, func(string) workspace.ChatEngine { return nil })
	appCfg := workspace.AppConfig{SupportedDomains: []string{"golang"}}

	mgmtAPI := api.NewManagementAPI(registry, manager, appCfg)
	mux := http.NewServeMux()
	mgmtAPI.RegisterHandlers(mux)

	// 2. Setup Operational Space (Valid state)
	opSpace := "sandbox-operational"
	_ = os.MkdirAll(filepath.Join(baseRoot, opSpace), 0755)
	_ = manager.UpdateSpaceState(opSpace, func(s *workspace.SpaceState) { s.Domain = "golang" })

	// 3. Setup Unconfigured Space (Missing state)
	unconfigSpace := "sandbox-unconfigured"
	_ = os.MkdirAll(filepath.Join(baseRoot, unconfigSpace), 0755)

	req := httptest.NewRequest(http.MethodGet, "/api/spaces", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var result []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 spaces, got %d", len(result))
	}

	for _, s := range result {
		id := s["id"].(string)
		isConfigured := s["is_configured"].(bool)

		if id == opSpace && !isConfigured {
			t.Errorf("expected operational space to be configured")
		}
		if id == unconfigSpace && isConfigured {
			t.Errorf("expected missing state space to be unconfigured")
		}
	}
}

func TestManagementAPI_CreateAndListChats(t *testing.T) {
	mux, _ := setupManagementAPI(t)
	spaceID := "sandbox"

	payload := `{"name": "REST Chat"}`
	req := httptest.NewRequest(http.MethodPost, "/api/spaces/"+spaceID+"/chats", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", rr.Code)
	}

	reqList := httptest.NewRequest(http.MethodGet, "/api/spaces/"+spaceID+"/chats", nil)
	rrList := httptest.NewRecorder()
	mux.ServeHTTP(rrList, reqList)

	if rrList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rrList.Code)
	}
}
