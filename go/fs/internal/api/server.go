package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"

	"github.com/coder/websocket"
	"google.golang.org/genai"

	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace/flows"
)

type Server struct {
	logger     *slog.Logger
	registry   *config.Registry
	manager    *workspace.ServiceManager
	llmAdapter *llm.Adapter
	flow       flows.Flow
	client     llm.ModelClient
}

func NewServer(
	logger *slog.Logger,
	registry *config.Registry,
	manager *workspace.ServiceManager,
	llmAdapter *llm.Adapter,
	flow flows.Flow,
	client llm.ModelClient,
) *Server {
	return &Server{
		logger:     logger,
		registry:   registry,
		manager:    manager,
		llmAdapter: llmAdapter,
		flow:       flow,
		client:     client,
	}
}

func (s *Server) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/receipts/{chat_id}/{flow_id}", s.handleGetReceipt)
	mux.HandleFunc("/ws", s.handleWebSocket)
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
	thread := &workspace.Thread{
		ID:  chatID,
		Dir: filepath.Join(service.WorkspaceRoot(), "chats", chatID),
	}

	data, err := service.GetReceipt(r.Context(), thread, flowID)
	if err != nil {
		http.Error(w, "Receipt not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	_, _ = w.Write(data)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	socket, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		s.logger.Error("WebSocket upgrade failed", "error", err)
		return
	}
	defer socket.Close(websocket.StatusNormalClosure, "session ended")

	ctx := r.Context()
	tokenChan := make(chan workspace.AgentToken, 100)

	ui := NewWebSocketUI(ctx, socket, tokenChan)
	facade := NewEventFacade()

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
		_ = socket.Write(ctx, websocket.MessageText, handshakeBytes)
	}

	go func() {
		for token := range tokenChan {
			if streamBytes, err := facade.MarshalAgentStream(token.AgentID, token.Text); err == nil {
				_ = socket.Write(ctx, websocket.MessageText, streamBytes)
			}
		}
	}()

	for {
		_, data, err := socket.Read(ctx)
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

			if payload.SpaceID == "" || payload.ChatID == "" {
				ui.OnTextChunk("⚠️ Server error: Missing space_id or chat_id in prompt.")
				continue
			}

			// Resolve domain dynamically based on physical space state
			spaceState, err := s.manager.GetSpaceState(payload.SpaceID)
			if err != nil || spaceState.Domain == "" {
				ui.OnTextChunk("⚠️ Server error: This ThinkSpace is missing its configuration state (space.json).")
				continue
			}

			activeDomain, ok := s.registry.GetDomain(spaceState.Domain)
			if !ok {
				ui.OnTextChunk(fmt.Sprintf("⚠️ Server error: The required domain template '%s' is missing from the global registry.", spaceState.Domain))
				continue
			}

			ui.OnTextChunk("\n🤖 The Manager is thinking...\n")

			flowConfig, flowOk := s.registry.GetFlow("fanout")
			if !flowOk {
				s.logger.Error("critical error: fanout flow configuration not found in registry")
				continue
			}

			go func(promptText string, domain workspace.ThinkSpace, spaceID string, chatID string, config flows.FlowConfig) {
				service := s.manager.GetService(spaceID)

				thread, err := service.StartThread(ctx, chatID)
				if err != nil {
					s.logger.Error("Failed to start thread", "error", err)
					return
				}

				_ = service.LogUserPrompt(ctx, thread, promptText)
				history := []*genai.Content{
					{Role: "user", Parts: []*genai.Part{{Text: promptText}}},
				}

				logEmitter := flows.NewSlogEmitter(s.logger)
				webSocketEmitter := NewWebSocketEmitter(ctx, socket, facade)
				multiEmitter := flows.MultiFlowEmitter{logEmitter, webSocketEmitter}

				workerModel := domain.Model(workspace.ModelCategoryWorker)
				workerRules := domain.SubAgentSystemPrompt()
				executor := llm.NewSubAgentExecutor(s.client, workerModel, workerRules)

				coordinator := session.NewCoordinator(
					s.logger,
					service,
					s.llmAdapter,
					executor,
					s.flow,
					multiEmitter,
					spaceID,
					workerRules,
					config,
				)

				if err := coordinator.ExecuteTurn(ctx, thread, domain, history, ui); err != nil {
					s.logger.Error("Coordinator execution failed", "error", err)
					ui.OnTextChunk(fmt.Sprintf("\n\n⚠️ System Error: %v\n", err))
				}
			}(payload.Text, activeDomain, payload.SpaceID, payload.ChatID, flowConfig)

		case "select_strategy":
			ui.PushStrategy(inboundEvent.SelectStrategy.StrategyID)

		case "review_decision":
			ui.PushReviewDecision(inboundEvent.ReviewDecision.Accepted)
		}
	}
}
