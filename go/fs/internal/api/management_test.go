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
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	baseRoot := t.TempDir()

	factory := func(repoRoot string) workspace.ChatVersioner { return nil }

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("failed to init badger: %v", err)
	}
	t.Cleanup(func() { _ = graphDB.Close() })

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

	// Empty-body POST creates a default-named chat cleanly
	emptyReq := httptest.NewRequest(http.MethodPost, "/api/spaces/"+spaceID+"/chats", nil)
	emptyRR := httptest.NewRecorder()
	mux.ServeHTTP(emptyRR, emptyReq)
	if emptyRR.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for empty-body chat creation, got %d", emptyRR.Code)
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

	t.Run("Resolves GraphID From ChatMeta", func(t *testing.T) {
		ctx := context.Background()
		meta, err := manager.CreateChat(ctx, spaceID, "GraphID Chat")
		if err != nil {
			t.Fatalf("CreateChat failed: %v", err)
		}
		gID := thinkgraph.NewGraphID(thinkgraph.PrefixChat)
		if err := manager.UpdateChatMeta(spaceID, meta.ID, func(m *workspace.ChatMeta) {
			m.BranchGraphIDs = map[string]string{"main": gID.String()}
		}); err != nil {
			t.Fatalf("UpdateChatMeta failed: %v", err)
		}

		scopedAddr := &thinkgraph.ChatAddress{
			ChatID:    meta.ID,
			ProjectID: spaceID,
			SubGraph:  thinkgraph.SubGraph{Name: "main", GraphID: gID},
		}
		ev := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Scoped GraphID prompt")
		ev.GraphID = gID
		if err := manager.GraphRepository().AppendNode(ctx, scopedAddr, thinkgraph.Node{Event: ev}); err != nil {
			t.Fatalf("AppendNode failed: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/spaces/"+spaceID+"/chats/"+meta.ID+"/events", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rr.Code)
		}
		var nodes []thinkgraph.Node
		if err := json.NewDecoder(rr.Body).Decode(&nodes); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		if len(nodes) != 1 || nodes[0].ID != ev.ID {
			t.Errorf("expected event %s from BranchGraphIDs, got %+v", ev.ID, nodes)
		}
	})
}

func TestManagementAPI_ContextsLensesAndEventMutations(t *testing.T) {
	mux, manager := setupManagementAPI(t)
	ctx := context.Background()
	spaceID := "sandbox"
	chatID := "chat-mut"

	// 1. Create and list SpaceContext
	ctxReq := httptest.NewRequest(http.MethodPost, "/api/spaces/"+spaceID+"/chats/"+chatID+"/contexts", bytes.NewBufferString(`{"content":"Use Go 1.24","tags":["go"]}`))
	ctxRR := httptest.NewRecorder()
	mux.ServeHTTP(ctxRR, ctxReq)
	if ctxRR.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for context, got %d", ctxRR.Code)
	}

	getCtxReq := httptest.NewRequest(http.MethodGet, "/api/spaces/"+spaceID+"/chats/"+chatID+"/contexts", nil)
	getCtxRR := httptest.NewRecorder()
	mux.ServeHTTP(getCtxRR, getCtxReq)
	if getCtxRR.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET contexts, got %d", getCtxRR.Code)
	}

	// 2. Query Lenses by tag
	lensReq := httptest.NewRequest(http.MethodGet, "/api/spaces/"+spaceID+"/chats/"+chatID+"/lenses/go", nil)
	lensRR := httptest.NewRecorder()
	mux.ServeHTTP(lensRR, lensReq)
	if lensRR.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET lenses, got %d", lensRR.Code)
	}
	var lensResult thinkgraph.LensResult
	if err := json.NewDecoder(lensRR.Body).Decode(&lensResult); err != nil {
		t.Fatalf("failed decoding lens result: %v", err)
	}
	if len(lensResult.SpaceContexts) != 1 || lensResult.SpaceContexts[0].Content != "Use Go 1.24" {
		t.Errorf("unexpected lens result: %+v", lensResult)
	}

	// 3. Tombstone, Restore, and Update event mutations
	chat := &thinkgraph.ChatAddress{ChatID: chatID, ProjectID: spaceID, SubGraph: thinkgraph.SubGraph{Name: "main"}}
	ev := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Original prompt")
	if err := manager.GraphRepository().AppendNode(ctx, chat, thinkgraph.Node{Event: ev}); err != nil {
		t.Fatalf("AppendNode failed: %v", err)
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/api/spaces/"+spaceID+"/chats/"+chatID+"/events/"+ev.ID.String(), nil)
	delRR := httptest.NewRecorder()
	mux.ServeHTTP(delRR, delReq)
	if delRR.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content on tombstone, got %d", delRR.Code)
	}

	restReq := httptest.NewRequest(http.MethodPost, "/api/spaces/"+spaceID+"/chats/"+chatID+"/events/"+ev.ID.String()+"/restore", nil)
	restRR := httptest.NewRecorder()
	mux.ServeHTTP(restRR, restReq)
	if restRR.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content on restore, got %d", restRR.Code)
	}

	updReq := httptest.NewRequest(http.MethodPut, "/api/spaces/"+spaceID+"/chats/"+chatID+"/events/"+ev.ID.String(), bytes.NewBufferString(`{"content":"Edited prompt"}`))
	updRR := httptest.NewRecorder()
	mux.ServeHTTP(updRR, updReq)
	if updRR.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content on update, got %d", updRR.Code)
	}

	nodes, err := manager.GraphRepository().GetNodes(ctx, chat, []string{ev.ID.String()})
	if err != nil || len(nodes) != 1 || nodes[0].Content != "Edited prompt" {
		t.Errorf("expected updated node content 'Edited prompt', got %+v (err=%v)", nodes, err)
	}
}
