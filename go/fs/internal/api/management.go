package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type ManagementAPI struct {
	registry *config.Registry
	manager  *workspace.ServiceManager
	appCfg   workspace.AppConfig
}

func NewManagementAPI(registry *config.Registry, manager *workspace.ServiceManager, appCfg workspace.AppConfig) *ManagementAPI {
	return &ManagementAPI{
		registry: registry,
		manager:  manager,
		appCfg:   appCfg,
	}
}

func (api *ManagementAPI) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/config", api.handleGetConfig)
	mux.HandleFunc("GET /api/spaces", api.handleGetSpaces)
	mux.HandleFunc("GET /api/spaces/{space_id}/chats", api.handleListChats)
	mux.HandleFunc("POST /api/spaces/{space_id}/chats", api.handleCreateChat)
	mux.HandleFunc("GET /api/spaces/{space_id}/chats/{chat_id}/events", api.handleGetEvents)

	mux.HandleFunc("GET /api/spaces/{space_id}/chats/{chat_id}/contexts", api.handleGetContexts)
	mux.HandleFunc("POST /api/spaces/{space_id}/chats/{chat_id}/contexts", api.handleCreateContext)
	mux.HandleFunc("GET /api/spaces/{space_id}/chats/{chat_id}/lenses/{tag}", api.handleGetLenses)

	mux.HandleFunc("DELETE /api/spaces/{space_id}/chats/{chat_id}/events/{id}", api.handleTombstoneEvent)
	mux.HandleFunc("POST /api/spaces/{space_id}/chats/{chat_id}/events/{id}/restore", api.handleRestoreEvent)
	mux.HandleFunc("PUT /api/spaces/{space_id}/chats/{chat_id}/events/{id}", api.handleUpdateEvent)
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

		if err == nil && state.ThinkState != "" {
			if _, ok := api.registry.GetThinkState(state.ThinkState); ok {
				isConfigured = true
			}
		}

		states = append(states, SpaceState{
			ID:           id,
			Name:         id,
			IsConfigured: isConfigured,
		})
	}

	responseBytes, err := MarshalRESTSpaces(states)
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

func (api *ManagementAPI) handleGetEvents(w http.ResponseWriter, r *http.Request) {
	spaceID := r.PathValue("space_id")
	chatID := r.PathValue("chat_id")
	idsParam := r.URL.Query().Get("ids")

	if spaceID == "" || chatID == "" {
		http.Error(w, "Missing space_id or chat_id parameter", http.StatusBadRequest)
		return
	}

	if api.manager.GraphRepository() == nil {
		http.Error(w, "Graph database is not initialized", http.StatusInternalServerError)
		return
	}

	meta, _ := api.manager.GetChatMeta(spaceID, chatID)
	activeBranch := "main"
	topo := make(map[string]thinkgraph.BranchTopology)
	if meta.ActiveBranch != "" {
		activeBranch = meta.ActiveBranch
		for k, v := range meta.Branches {
			topo[k] = thinkgraph.BranchTopology{ParentBranch: v.ParentBranch, ForkPoint: v.ForkPoint}
		}
	}

	chat := &thinkgraph.ChatAddress{
		ChatID:    chatID,
		ProjectID: spaceID,
		SubGraph: thinkgraph.SubGraph{
			Name: activeBranch,
		},
	}
	var events []thinkgraph.Node
	var err error

	if idsParam != "" {
		ids := strings.Split(idsParam, ",")
		events, err = api.manager.GraphRepository().GetNodes(r.Context(), chat, ids)
	} else {
		limit := 50
		if l, parseErr := strconv.Atoi(r.URL.Query().Get("limit")); parseErr == nil && l > 0 {
			limit = l
		}
		cursor := r.URL.Query().Get("cursor")

		var timeline []thinkgraph.Node
		engine := thinkgraph.NewTraversalEngine(api.manager.GraphRepository())
		timeline, err = engine.GetFullTimeline(r.Context(), chat, topo)

		if err == nil {
			endIdx := len(timeline)
			if cursor != "" {
				for i, n := range timeline {
					if n.ID.String() == cursor {
						endIdx = i
						break
					}
				}
			}

			startIdx := endIdx - limit
			if startIdx < 0 {
				startIdx = 0
			}

			if startIdx < endIdx {
				events = timeline[startIdx:endIdx]
			} else {
				events = []thinkgraph.Node{}
			}
		}
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(events); err != nil {
		http.Error(w, "Failed to encode events", http.StatusInternalServerError)
	}
}

func (api *ManagementAPI) handleGetContexts(w http.ResponseWriter, r *http.Request) {
	spaceID := r.PathValue("space_id")
	chatID := r.PathValue("chat_id")

	if spaceID == "" || chatID == "" {
		http.Error(w, "Missing space_id or chat_id parameter", http.StatusBadRequest)
		return
	}

	if api.manager.GraphRepository() == nil {
		http.Error(w, "Graph database is not initialized", http.StatusInternalServerError)
		return
	}

	chat := &thinkgraph.ChatAddress{ChatID: chatID, ProjectID: spaceID}

	contexts, err := api.manager.GraphRepository().GetSpaceContexts(r.Context(), chat)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(contexts); err != nil {
		http.Error(w, "Failed to encode contexts", http.StatusInternalServerError)
	}
}

func (api *ManagementAPI) handleCreateContext(w http.ResponseWriter, r *http.Request) {
	spaceID := r.PathValue("space_id")
	chatID := r.PathValue("chat_id")

	if spaceID == "" || chatID == "" {
		http.Error(w, "Missing space_id or chat_id parameter", http.StatusBadRequest)
		return
	}

	var payload struct {
		Content string   `json:"content"`
		Tags    []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	svc := api.manager.GetService(spaceID)
	chat := &thinkgraph.ChatAddress{ChatID: chatID, ProjectID: spaceID}

	sc := thinkgraph.SpaceContext{
		ID:      uuid.NewV7(),
		Content: payload.Content,
		Tags:    payload.Tags,
	}

	if err := svc.SaveSpaceContext(r.Context(), chat, sc); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(sc); err != nil {
		http.Error(w, "Failed to encode created context", http.StatusInternalServerError)
	}
}

func (api *ManagementAPI) handleGetLenses(w http.ResponseWriter, r *http.Request) {
	spaceID := r.PathValue("space_id")
	chatID := r.PathValue("chat_id")
	tag := r.PathValue("tag")

	if spaceID == "" || chatID == "" || tag == "" {
		http.Error(w, "Missing space_id, chat_id, or tag parameter", http.StatusBadRequest)
		return
	}

	if api.manager.GraphRepository() == nil {
		http.Error(w, "Graph database is not initialized", http.StatusInternalServerError)
		return
	}

	chat := &thinkgraph.ChatAddress{ChatID: chatID, ProjectID: spaceID}

	result, err := api.manager.GraphRepository().GetLenses(r.Context(), chat, tag)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, "Failed to encode lens result", http.StatusInternalServerError)
	}
}

func (api *ManagementAPI) handleTombstoneEvent(w http.ResponseWriter, r *http.Request) {
	spaceID := r.PathValue("space_id")
	chatID := r.PathValue("chat_id")
	targetIDStr := r.PathValue("id")

	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		http.Error(w, "Invalid target ID", http.StatusBadRequest)
		return
	}

	svc := api.manager.GetService(spaceID)
	chat := &thinkgraph.ChatAddress{ChatID: chatID, ProjectID: spaceID}

	mut := thinkgraph.MutationEvent{
		TargetID:  targetID,
		Type:      thinkgraph.MutationTombstone,
		Timestamp: time.Now().UTC(),
	}

	if err := svc.SaveMutation(r.Context(), chat, mut); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (api *ManagementAPI) handleRestoreEvent(w http.ResponseWriter, r *http.Request) {
	spaceID := r.PathValue("space_id")
	chatID := r.PathValue("chat_id")
	targetIDStr := r.PathValue("id")

	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		http.Error(w, "Invalid target ID", http.StatusBadRequest)
		return
	}

	svc := api.manager.GetService(spaceID)
	chat := &thinkgraph.ChatAddress{ChatID: chatID, ProjectID: spaceID}

	mut := thinkgraph.MutationEvent{
		TargetID:  targetID,
		Type:      thinkgraph.MutationRestore,
		Timestamp: time.Now().UTC(),
	}

	if err := svc.SaveMutation(r.Context(), chat, mut); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (api *ManagementAPI) handleUpdateEvent(w http.ResponseWriter, r *http.Request) {
	spaceID := r.PathValue("space_id")
	chatID := r.PathValue("chat_id")
	targetIDStr := r.PathValue("id")

	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		http.Error(w, "Invalid target ID", http.StatusBadRequest)
		return
	}

	var payload struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	svc := api.manager.GetService(spaceID)
	chat := &thinkgraph.ChatAddress{ChatID: chatID, ProjectID: spaceID}

	mut := thinkgraph.MutationEvent{
		TargetID:  targetID,
		Type:      thinkgraph.MutationUpdate,
		Timestamp: time.Now().UTC(),
		Payload:   payload.Content,
	}

	if err := svc.SaveMutation(r.Context(), chat, mut); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
