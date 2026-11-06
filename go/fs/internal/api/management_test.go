package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/tinywideclouds.com/thinkspace/internal/api"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/db"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

func setupManagementAPI(t *testing.T) (*http.ServeMux, *workspace.ServiceManager) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	baseRoot := t.TempDir()

	factory := func(repoRoot string) workspace.ChatVersioner { return nil }

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, _ := db.NewBadgerGraph(opts)
	t.Cleanup(func() { graphDB.Close() })

	manager := workspace.NewServiceManager(logger, baseRoot, factory, graphDB, nil)

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

	yamlContent := `golang:
  type: thinkstate
  name: "Golang Domain"
`
	_ = os.WriteFile(filepath.Join(configsDir, "golang.yaml"), []byte(yamlContent), 0644)
	registry := config.NewRegistry()
	_ = registry.LoadDirectory(configsDir)

	manager := workspace.NewServiceManager(logger, baseRoot, func(string) workspace.ChatVersioner { return nil }, nil, nil)
	appCfg := workspace.AppConfig{SupportedDomains: []string{"golang"}}

	mgmtAPI := api.NewManagementAPI(registry, manager, appCfg)
	mux := http.NewServeMux()
	mgmtAPI.RegisterHandlers(mux)

	opSpace := "sandbox-operational"
	_ = os.MkdirAll(filepath.Join(baseRoot, opSpace), 0755)
	_ = manager.UpdateSpaceState(opSpace, func(s *workspace.SpaceState) { s.ThinkState = "golang" })

	unconfigSpace := "sandbox-unconfigured"
	_ = os.MkdirAll(filepath.Join(baseRoot, unconfigSpace), 0755)

	req := httptest.NewRequest(http.MethodGet, "/api/spaces", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	spaces, err := api.UnmarshalRESTSpaces(rr.Body.Bytes())
	if err != nil {
		t.Fatalf("failed to unmarshal response via facade: %v", err)
	}

	if len(spaces) != 2 {
		t.Fatalf("expected 2 spaces, got %d", len(spaces))
	}

	for _, s := range spaces {
		if s.ID == opSpace && !s.IsConfigured {
			t.Errorf("expected operational space to be configured")
		}
		if s.ID == unconfigSpace && s.IsConfigured {
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

func TestManagementAPI_GetEvents(t *testing.T) {
	mux, manager := setupManagementAPI(t)
	spaceID := "sandbox"
	chatID := "chat-1"

	chat := &thinkgraph.ChatAddress{
		ChatID:    chatID,
		ProjectID: spaceID,
		SubGraph: thinkgraph.SubGraph{
			Name: "main",
		},
	}

	event1 := thinkgraph.NewEvent(thinkgraph.EventPrompt, "First test event")
	event1.Timestamp = time.Now().Add(-2 * time.Hour)
	event2 := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Second test event")
	event2.Timestamp = time.Now().Add(-1 * time.Hour)
	event3 := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Third test event")
	event3.Timestamp = time.Now()

	_ = manager.GraphRepository().AppendNode(context.Background(), chat, thinkgraph.Node{Event: event1})
	_ = manager.GraphRepository().AppendNode(context.Background(), chat, thinkgraph.Node{Event: event2})
	_ = manager.GraphRepository().AppendNode(context.Background(), chat, thinkgraph.Node{Event: event3})

	t.Run("Fetch By Explicit IDs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/spaces/"+spaceID+"/chats/"+chatID+"/events?ids="+event1.ID.String(), nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rr.Code)
		}

		var nodes []thinkgraph.Node
		if err := json.NewDecoder(rr.Body).Decode(&nodes); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(nodes) != 1 || nodes[0].ID != event1.ID {
			t.Errorf("Expected only event1 via ID fetch")
		}
	})

	t.Run("Paginate Timeline Limit Only", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/spaces/"+spaceID+"/chats/"+chatID+"/events?limit=2", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rr.Code)
		}

		var nodes []thinkgraph.Node
		_ = json.NewDecoder(rr.Body).Decode(&nodes)

		if len(nodes) != 2 {
			t.Fatalf("Expected exactly 2 events limited, got %d", len(nodes))
		}
		if nodes[0].ID != event2.ID || nodes[1].ID != event3.ID {
			t.Errorf("Expected most recent events (event2, event3)")
		}
	})

	t.Run("Paginate Timeline With Cursor", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/spaces/"+spaceID+"/chats/"+chatID+"/events?limit=2&cursor="+event3.ID.String(), nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		var nodes []thinkgraph.Node
		_ = json.NewDecoder(rr.Body).Decode(&nodes)

		if len(nodes) != 2 {
			t.Fatalf("Expected exactly 2 events limited before cursor, got %d", len(nodes))
		}
		if nodes[0].ID != event1.ID || nodes[1].ID != event2.ID {
			t.Errorf("Expected events before cursor (event1, event2)")
		}
	})
}
