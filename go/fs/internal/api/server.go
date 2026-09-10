package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"

	"github.com/coder/websocket"
	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type Server struct {
	logger      *slog.Logger
	registry    *config.Registry
	manager     *workspace.ServiceManager
	turnManager *session.TurnManager
}

func NewServer(
	logger *slog.Logger,
	registry *config.Registry,
	manager *workspace.ServiceManager,
	llmAdapter *llm.Adapter,
	fanOutFlow flows.Flow,
	modelClient llm.ModelClient,
) *Server {
	return &Server{
		logger:      logger,
		registry:    registry,
		manager:     manager,
		turnManager: session.NewTurnManager(logger, registry, manager, llmAdapter, fanOutFlow, modelClient),
	}
}

func (s *Server) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/receipts/{chat_id}/{flow_id}", s.handleGetReceipt)
	mux.HandleFunc("/ws", s.HandleWebSocket)
}

func (s *Server) handleGetReceipt(w http.ResponseWriter, r *http.Request) {
	chatID := r.PathValue("chat_id")
	flowID := r.PathValue("flow_id")
	spaceID := r.URL.Query().Get("space")

	if spaceID == "" {
		http.Error(w, "Missing space query parameter", http.StatusBadRequest)
		return
	}

	service := s.manager.GetService(spaceID)
	thread := &chat.Thread{
		ID:      chatID,
		SpaceID: spaceID,
		Dir:     filepath.Join(service.WorkspaceRoot(), "chats", chatID),
	}

	data, err := service.GetReceipt(r.Context(), thread, flowID)
	if err != nil {
		http.Error(w, "Receipt not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	_, _ = w.Write(data)
}

func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	connection, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		s.logger.Error("WebSocket upgrade failed", "error", err)
		return
	}
	defer connection.Close(websocket.StatusInternalError, "closing connection")

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// 1. Restore the Token Streaming Channel for Sub-Agents
	tokenChan := make(chan workspace.AgentToken, 100)
	facade := NewEventFacade()
	ui := NewWebSocketUI(ctx, connection, tokenChan)

	// 2. Restore the Initial Handshake (Available Spaces)
	spaceIDs, _ := s.manager.ListSpaces(ctx)
	var availableSpaces []SpaceState

	for _, id := range spaceIDs {
		if id == "configs" {
			continue
		}

		isConfigured := false
		state, err := s.manager.GetSpaceState(id)

		if err == nil && state.Domain != "" {
			if _, ok := s.registry.GetDomain(state.Domain); ok {
				isConfigured = true
			}
		}

		availableSpaces = append(availableSpaces, SpaceState{
			ID:           id,
			Name:         id,
			IsConfigured: isConfigured,
		})
	}

	if handshakeBytes, err := facade.MarshalAvailableSpaces(availableSpaces); err == nil {
		_ = connection.Write(ctx, websocket.MessageText, handshakeBytes)
	}

	// 3. Restore the Async Token Streamer
	go func() {
		for token := range tokenChan {
			if streamBytes, err := facade.MarshalAgentStream(token.AgentID, token.Text); err == nil {
				_ = connection.Write(ctx, websocket.MessageText, streamBytes)
			}
		}
	}()

	// 4. The Main Event Loop
	for {
		_, msgBytes, err := connection.Read(ctx)
		if err != nil {
			break // Client disconnected
		}

		inbound, err := facade.UnmarshalInbound(msgBytes)
		if err != nil {
			s.logger.Warn("invalid websocket message", "error", err)
			continue
		}

		switch inbound.Type {
		case "submit_prompt":
			payload := inbound.SubmitPrompt

			// UI Hydration using the current TurnManager
			_, graph, manifest, err := s.turnManager.LoadThreadState(ctx, payload.SpaceID, payload.ChatID)
			if err == nil {
				if syncData, err := facade.MarshalSyncHistory(graph.RecentEvents, manifest.Digests); err == nil {
					ui.SendSyncHistory(syncData)
				}
			}

			if payload.Text == "" {
				continue
			}

			// Execution using the current TurnManager
			emitter := NewWebSocketEmitter(ctx, connection, facade)
			go func() {
				if err := s.turnManager.ExecuteTurn(ctx, payload.SpaceID, payload.ChatID, payload.Text, ui, emitter); err != nil {
					s.logger.Error("turn failed", "error", err)
					ui.OnTextChunk(fmt.Sprintf("\n\n⚠️ System Error: %v\n", err))
				}
			}()

		// 5. Restore the Missing Interactive UI Routing
		case "select_strategy":
			ui.PushStrategy(inbound.SelectStrategy.StrategyID)

		case "review_decision":
			ui.PushReviewDecision(inbound.ReviewDecision.Accepted)
		}
	}
}
