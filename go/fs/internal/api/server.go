package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/coder/websocket"
	"google.golang.org/genai"

	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace/flows"
)

type Server struct {
	logger   *slog.Logger
	registry *config.Registry
	service  *workspace.Service
	llmMgr   *llm.Manager
	flow     flows.Flow
	client   llm.ModelClient
}

func NewServer(
	logger *slog.Logger,
	registry *config.Registry,
	service *workspace.Service,
	llmMgr *llm.Manager,
	flow flows.Flow,
	client llm.ModelClient,
) *Server {
	return &Server{
		logger:   logger,
		registry: registry,
		service:  service,
		llmMgr:   llmMgr,
		flow:     flow,
		client:   client,
	}
}

// Handler returns the HTTP router with all endpoints registered.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/spaces", s.handleGetSpaces)
	mux.HandleFunc("/ws", s.handleWebSocket)
	return mux
}

func (s *Server) handleGetSpaces(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.registry.GetAllConfigs())
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		s.logger.Error("WebSocket upgrade failed", "error", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "session ended")

	wsCtx := r.Context()
	tokenChan := make(chan workspace.AgentToken, 100)

	ui := NewWebSocketUI(wsCtx, conn, tokenChan)
	facade := NewEventFacade()

	// Handshake: Emit Available Spaces safely using the Facade
	var availableSpaces []SpaceInfo
	for _, space := range s.registry.GetAvailableSpaces() {
		availableSpaces = append(availableSpaces, SpaceInfo{ID: space.ID, Name: space.Name})
	}

	if handshakeBytes, err := facade.MarshalAvailableSpaces(availableSpaces); err == nil {
		_ = conn.Write(wsCtx, websocket.MessageText, handshakeBytes)
	}

	// Multiplexing Goroutine safely using the Facade
	go func() {
		for token := range tokenChan {
			if streamBytes, err := facade.MarshalAgentStream(token.AgentID, token.Text); err == nil {
				_ = conn.Write(wsCtx, websocket.MessageText, streamBytes)
			}
		}
	}()

	// Inbound Message Loop
	for {
		_, data, err := conn.Read(wsCtx)
		if err != nil {
			s.logger.Info("WebSocket client disconnected")
			break
		}

		inboundEvent, err := facade.UnmarshalInbound(data)
		if err != nil {
			s.logger.Error("Failed to unmarshal inbound message", "error", err)
			continue
		}

		switch inboundEvent.Type {
		case "submit_prompt":
			payload := inboundEvent.SubmitPrompt
			activeSpace, ok := s.registry.GetSpace(payload.SpaceID)
			if !ok {
				ui.OnTextChunk(fmt.Sprintf("⚠️ Server error: Space '%s' not found.", payload.SpaceID))
				continue
			}

			go func(promptText string, space workspace.ThinkSpace) {
				chatName := "unit-circle-test"
				thread, err := s.service.StartThread(wsCtx, chatName)
				if err != nil {
					s.logger.Error("Failed to start thread", "error", err)
					return
				}

				_ = s.service.LogUserPrompt(wsCtx, thread, promptText)
				history := []*genai.Content{
					{Role: "user", Parts: []*genai.Part{{Text: promptText}}},
				}

				workerModel := space.Model(workspace.ModelCategoryWorker)
				executor := llm.SubAgentFactory(s.client, workerModel)
				coordinator := session.NewCoordinator(s.logger, s.service, s.llmMgr, executor, s.flow)

				_ = coordinator.ExecuteTurn(wsCtx, thread, space, history, ui)
			}(payload.Text, activeSpace)

		case "select_strategy":
			ui.PushStrategy(inboundEvent.SelectStrategy.StrategyID)

		case "review_decision":
			ui.PushReviewDecision(inboundEvent.ReviewDecision.Accepted)
		}
	}
}
