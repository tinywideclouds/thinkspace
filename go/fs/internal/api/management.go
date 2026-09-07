package api

import (
	"encoding/json"
	"net/http"

	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type ManagementAPI struct {
	registry *config.Registry
	manager  *workspace.ServiceManager
	appCfg   workspace.AppConfig
	facade   *EventFacade
}

func NewManagementAPI(registry *config.Registry, manager *workspace.ServiceManager, appCfg workspace.AppConfig) *ManagementAPI {
	return &ManagementAPI{
		registry: registry,
		manager:  manager,
		appCfg:   appCfg,
		facade:   NewEventFacade(),
	}
}

func (api *ManagementAPI) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/config", api.handleGetConfig)
	mux.HandleFunc("GET /api/spaces", api.handleGetSpaces)
	mux.HandleFunc("GET /api/spaces/{space_id}/chats", api.handleListChats)
	mux.HandleFunc("POST /api/spaces/{space_id}/chats", api.handleCreateChat)
}

func (api *ManagementAPI) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(api.appCfg); err != nil {
		http.Error(w, "Failed to encode config", http.StatusInternalServerError)
	}
}

func (api *ManagementAPI) handleGetSpaces(w http.ResponseWriter, r *http.Request) {
	spaceIDs, err := api.manager.ListSpaces(r.Context())
	if err != nil {
		http.Error(w, "Failed to list spaces", http.StatusInternalServerError)
		return
	}

	var states []SpaceState
	for _, id := range spaceIDs {
		if id == "configs" {
			continue
		}

		isConfigured := false
		state, err := api.manager.GetSpaceState(id)

		// A space is operational if its requested domain exists in the loaded registry
		if err == nil && state.Domain != "" {
			if _, ok := api.registry.GetDomain(state.Domain); ok {
				isConfigured = true
			}
		}

		states = append(states, SpaceState{
			ID:           id,
			Name:         id,
			IsConfigured: isConfigured,
		})
	}

	responseBytes, err := api.facade.MarshalRESTSpaces(states)
	if err != nil {
		http.Error(w, "Failed to marshal spaces", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(json.RawMessage(responseBytes)); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (api *ManagementAPI) handleListChats(w http.ResponseWriter, r *http.Request) {
	spaceID := r.PathValue("space_id")
	if spaceID == "" {
		http.Error(w, "Missing space_id", http.StatusBadRequest)
		return
	}

	chats, err := api.manager.ListChats(r.Context(), spaceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(chats); err != nil {
		http.Error(w, "Failed to encode chats", http.StatusInternalServerError)
	}
}

func (api *ManagementAPI) handleCreateChat(w http.ResponseWriter, r *http.Request) {
	spaceID := r.PathValue("space_id")
	if spaceID == "" {
		http.Error(w, "Missing space_id", http.StatusBadRequest)
		return
	}

	var payload struct {
		Name string `json:"name"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}
	}

	meta, err := api.manager.CreateChat(r.Context(), spaceID, payload.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(meta); err != nil {
		http.Error(w, "Failed to encode chat meta", http.StatusInternalServerError)
	}
}
