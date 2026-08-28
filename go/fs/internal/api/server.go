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
	logger      *slog.Logger
	registry    *config.Registry
	service     *workspace.Service
	llmMgr      *llm.Manager
	flow        flows.Flow
	client      llm.ModelClient
	initialChat string
}

func NewServer(
	logger *slog.Logger,
	registry *config.Registry,
	service *workspace.Service,
	llmMgr *llm.Manager,
	flow flows.Flow,
	client llm.ModelClient,
	initialChat string,
) *Server {
	return &Server{
		logger:      logger,
		registry:    registry,
		service:     service,
		llmMgr:      llmMgr,
		flow:        flow,
		client:      client,
		initialChat: initialChat,
	}
}

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

	var availableSpaces []SpaceInfo
	for _, space := range s.registry.GetAvailableSpaces() {
		availableSpaces = append(availableSpaces, SpaceInfo{ID: space.ID, Name: space.Name})
	}

	if handshakeBytes, err := facade.MarshalAvailableSpaces(availableSpaces); err == nil {
		_ = conn.Write(wsCtx, websocket.MessageText, handshakeBytes)
	}

	go func() {
		for token := range tokenChan {
			if streamBytes, err := facade.MarshalAgentStream(token.AgentID, token.Text); err == nil {
				_ = conn.Write(wsCtx, websocket.MessageText, streamBytes)
			}
		}
	}()

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

			spaceConfig, _ := s.registry.GetConfig(payload.SpaceID)
			flowCfg, flowOk := s.registry.GetFlow("fanout")
			if !flowOk {
				s.logger.Error("critical error: fanout flow configuration not found in registry")
				continue
			}

			go func(promptText string, space workspace.ThinkSpace, spaceID string, baseRules string, cfg flows.FlowConfig) {
				chatName := s.initialChat
				thread, err := s.service.StartThread(wsCtx, chatName)
				if err != nil {
					s.logger.Error("Failed to start thread", "error", err)
					return
				}

				_ = s.service.LogUserPrompt(wsCtx, thread, promptText)
				history := []*genai.Content{
					{Role: "user", Parts: []*genai.Part{{Text: promptText}}},
				}

				slogEmitter := flows.NewSlogEmitter(s.logger)
				wsEmitter := NewWebSocketEmitter(wsCtx, conn, facade)
				multiEmitter := flows.MultiFlowEmitter{slogEmitter, wsEmitter}

				workerModel := space.Model(workspace.ModelCategoryWorker)
				executor := llm.SubAgentFactory(s.client, workerModel)

				coordinator := session.NewCoordinator(
					s.logger,
					s.service,
					s.llmMgr,
					executor,
					s.flow,
					multiEmitter,
					spaceID,
					baseRules,
					cfg,
				)

				_ = coordinator.ExecuteTurn(wsCtx, thread, space, history, ui)
			}(payload.Text, activeSpace, payload.SpaceID, spaceConfig.BaseAgentRules, flowCfg)

		case "select_strategy":
			ui.PushStrategy(inboundEvent.SelectStrategy.StrategyID)

		case "review_decision":
			ui.PushReviewDecision(inboundEvent.ReviewDecision.Accepted)
		}
	}
}
