package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"uuid"
)

type Server struct {
	logger      *slog.Logger
	registry    *config.Registry
	manager     *workspace.ServiceManager
	turnManager *session.Manager
}

func NewServer(
	logger *slog.Logger,
	registry *config.Registry,
	manager *workspace.ServiceManager,
	turnManager *session.Manager,
) *Server {
	return &Server{
		logger:      logger,
		registry:    registry,
		manager:     manager,
		turnManager: turnManager,
	}
}

func (server *Server) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/previews/{chat_id}/{preview_id}", server.handleGetPreview)
	mux.HandleFunc("/ws", server.HandleWebSocket)
}

func (server *Server) handleGetPreview(w http.ResponseWriter, r *http.Request) {
	chatID := r.PathValue("chat_id")
	previewID := r.PathValue("preview_id")
	spaceID := r.URL.Query().Get("space")

	if spaceID == "" {
		http.Error(w, "Missing space query parameter", http.StatusBadRequest)
		return
	}

	service := server.manager.GetService(spaceID)
	chatSession := &thinkgraph.Chat{
		ID:      chatID,
		SpaceID: spaceID,
	}

	data, err := service.GetContextPreview(r.Context(), chatSession, previewID)
	if err != nil {
		http.Error(w, "Preview not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/markdown")
	if _, err := w.Write(data); err != nil {
		server.logger.WarnContext(r.Context(), "failed writing preview response", "error", err)
	}
}

func (server *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	connection, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		server.logger.Error("WebSocket upgrade failed", "error", err)
		return
	}
	defer connection.Close(websocket.StatusInternalError, "closing connection")

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	tokenChan := make(chan workspace.AgentToken, 100)
	ui := NewWebSocketUI(ctx, connection, tokenChan, server.logger)

	spaceIDs, err := server.manager.ListSpaces(ctx)
	if err != nil {
		server.logger.Error("Failed to list spaces during handshake", "error", err)
	}

	var availableSpaces []SpaceState
	for _, id := range spaceIDs {
		if id == "configs" {
			continue
		}

		isConfigured := false
		state, err := server.manager.GetSpaceState(id)

		if err == nil && state.ThinkState != "" {
			if _, ok := server.registry.GetThinkState(state.ThinkState); ok {
				isConfigured = true
			}
		}

		availableSpaces = append(availableSpaces, SpaceState{
			ID:           id,
			Name:         id,
			IsConfigured: isConfigured,
		})
	}

	handshakeBytes, err := MarshalAvailableSpaces(availableSpaces)
	if err != nil {
		server.logger.Error("Failed to marshal available spaces", "error", err)
	} else {
		if writeErr := connection.Write(ctx, websocket.MessageText, handshakeBytes); writeErr != nil {
			server.logger.Warn("Failed writing available spaces handshake", "error", writeErr)
		}
	}

	go func() {
		for token := range tokenChan {
			streamBytes, err := MarshalAgentStream(token.AgentID, token.Text)
			if err != nil {
				server.logger.Error("Failed marshaling agent stream", "error", err)
				continue
			}
			if writeErr := connection.Write(ctx, websocket.MessageText, streamBytes); writeErr != nil {
				server.logger.Warn("Failed streaming agent token to client", "error", writeErr)
			}
		}
	}()

	for {
		_, msgBytes, err := connection.Read(ctx)
		if err != nil {
			server.logger.Info("WebSocket client disconnected or read failed", "error", err)
			break
		}

		inbound, err := UnmarshalInbound(msgBytes)
		if err != nil {
			server.logger.Warn("invalid websocket message", "error", err)
			continue
		}

		switch inbound.Type {
		case "submit_prompt":
			payload := inbound.SubmitPrompt

			_, err := server.turnManager.GetSession(ctx, payload.SpaceID, payload.ChatID)
			if err != nil {
				server.logger.Error("failed to get chat session", "space_id", payload.SpaceID, "chat_id", payload.ChatID, "error", err)
				ui.OnTextChunk(fmt.Sprintf("\n\n⚠️ Failed to open chat session: %v\n", err))
				continue
			}

			if payload.Text == "" {
				continue
			}

			emitter := NewWebSocketEmitter(ctx, connection)
			go func(promptText string) {
				server.logger.Info("Starting RunTurn goroutine", "space_id", payload.SpaceID, "chat_id", payload.ChatID, "prompt", promptText)
				if err := server.turnManager.RunTurn(ctx, payload.SpaceID, payload.ChatID, promptText, ui, emitter); err != nil {
					server.logger.Error("turn failed in execution", "space_id", payload.SpaceID, "chat_id", payload.ChatID, "error", err)
					ui.OnTextChunk(fmt.Sprintf("\n\n⚠️ System Error: %v\n", err))
				} else {
					server.logger.Info("Finished RunTurn goroutine successfully", "space_id", payload.SpaceID, "chat_id", payload.ChatID)
				}
			}(payload.Text)

		case "load_history":
			payload := inbound.LoadHistory

			chatSess, err := server.turnManager.GetSession(ctx, payload.SpaceID, payload.ChatID)
			if err != nil {
				server.logger.Error("Failed retrieving session for history", "error", err)
				continue
			}

			chatSess.Mu().RLock()
			activeBranch := chatSess.Meta.ActiveBranch
			topo := make(map[string]thinkgraph.BranchTopology)
			for k, v := range chatSess.Meta.Branches {
				topo[k] = thinkgraph.BranchTopology{ParentBranch: v.ParentBranch, ForkPoint: v.ForkPoint}
			}
			chatSess.Mu().RUnlock()

			chatObj := &thinkgraph.Chat{ID: payload.ChatID, SpaceID: payload.SpaceID, ActiveBranch: activeBranch}
			limit := payload.Limit
			if limit <= 0 {
				limit = 50
			}

			engine := thinkgraph.NewTraversalEngine(server.manager.GraphRepository())
			events, nextCursor, hasMore, err := engine.GetTimelinePage(ctx, chatObj, payload.Cursor, limit, topo)
			if err != nil {
				server.logger.Error("Failed to fetch timeline page", "error", err)
				continue
			}

			var digests []thinkgraph.DigestMeta
			var focus thinkgraph.AttentionFocus

			// Piggyback full cognitive state if the client is fetching the initial timeline page
			if payload.Cursor == "" {
				if graphData, err := chatSess.GetGraphData(ctx); err == nil {
					digests = graphData.Digests
					focus = graphData.Focus
				} else {
					server.logger.Warn("Failed getting graph data for initial history load", "error", err)
				}
			}

			syncData, err := MarshalSyncHistory(events, digests, focus, nextCursor, hasMore)
			if err != nil {
				server.logger.Error("Failed to marshal history sync payload", "error", err)
				continue
			}
			ui.SendSyncHistory(syncData)

		case "fork_chat":
			payload := inbound.ForkChat

			chatSess, err := server.turnManager.GetSession(ctx, payload.SpaceID, payload.ChatID)
			if err != nil {
				server.logger.Error("Failed retrieving session for fork", "error", err)
				continue
			}

			chatSess.Mu().Lock()
			id := uuid.NewV7()
			newBranchID := "branch-" + id.String()[:8]

			chatSess.Meta.Branches[newBranchID] = workspace.BranchMeta{
				ParentBranch: chatSess.Meta.ActiveBranch,
				ForkPoint:    payload.TargetEventID,
			}
			chatSess.Meta.ActiveBranch = newBranchID

			topo := make(map[string]thinkgraph.BranchTopology)
			for k, v := range chatSess.Meta.Branches {
				topo[k] = thinkgraph.BranchTopology{ParentBranch: v.ParentBranch, ForkPoint: v.ForkPoint}
			}
			chatSess.Mu().Unlock()

			if err := server.manager.UpdateChatMeta(payload.SpaceID, payload.ChatID, func(m *workspace.ChatMeta) {
				m.Branches[newBranchID] = workspace.BranchMeta{ParentBranch: m.ActiveBranch, ForkPoint: payload.TargetEventID}
				m.ActiveBranch = newBranchID
			}); err != nil {
				server.logger.Error("Failed to update chat meta on fork", "error", err)
			}

			chatObj := &thinkgraph.Chat{ID: payload.ChatID, SpaceID: payload.SpaceID, ActiveBranch: newBranchID}
			engine := thinkgraph.NewTraversalEngine(server.manager.GraphRepository())

			events, nextCursor, hasMore, err := engine.GetTimelinePage(ctx, chatObj, "", 50, topo)
			if err != nil {
				server.logger.Error("Failed to get timeline after fork", "error", err)
			} else if syncData, err := MarshalSyncHistory(events, nil, thinkgraph.AttentionFocus{}, nextCursor, hasMore); err == nil {
				ui.SendSyncHistory(syncData)
			}

		case "inline_edit":
			payload := inbound.InlineEdit

			chatSess, err := server.turnManager.GetSession(ctx, payload.SpaceID, payload.ChatID)
			if err != nil {
				server.logger.Error("Failed retrieving session for inline edit", "error", err)
				continue
			}

			chatSess.Mu().RLock()
			activeBranch := chatSess.Meta.ActiveBranch
			chatSess.Mu().RUnlock()

			chatObj := &thinkgraph.Chat{ID: payload.ChatID, SpaceID: payload.SpaceID, ActiveBranch: activeBranch}

			if err := server.manager.GetService(payload.SpaceID).LogUserPrompt(ctx, chatObj, payload.NewText); err != nil {
				server.logger.Error("Failed logging amended prompt", "error", err)
			}

			if targetUUID, err := uuid.Parse(payload.TargetEventID); err != nil {
				server.logger.Error("Failed parsing target event ID for inline edit", "target_id", payload.TargetEventID, "error", err)
			} else {
				mut := thinkgraph.MutationEvent{
					TargetID:  targetUUID,
					Type:      thinkgraph.MutationUpdate,
					Timestamp: time.Now().UTC(),
					Payload:   payload.NewText,
				}
				if err := server.manager.GetService(payload.SpaceID).SaveMutation(ctx, chatObj, mut); err != nil {
					server.logger.Error("Failed saving mutation for inline edit", "error", err)
				}
			}

		case "switch_branch":
			payload := inbound.SwitchBranch

			chatSess, err := server.turnManager.GetSession(ctx, payload.SpaceID, payload.ChatID)
			if err != nil {
				server.logger.Error("Failed retrieving session for switch branch", "error", err)
				continue
			}

			chatSess.Mu().Lock()
			chatSess.Meta.ActiveBranch = payload.TargetBranch
			topo := make(map[string]thinkgraph.BranchTopology)
			for k, v := range chatSess.Meta.Branches {
				topo[k] = thinkgraph.BranchTopology{ParentBranch: v.ParentBranch, ForkPoint: v.ForkPoint}
			}
			chatSess.Mu().Unlock()

			if err := server.manager.UpdateChatMeta(payload.SpaceID, payload.ChatID, func(m *workspace.ChatMeta) {
				m.ActiveBranch = payload.TargetBranch
			}); err != nil {
				server.logger.Error("Failed updating meta on switch branch", "error", err)
			}

			chatObj := &thinkgraph.Chat{ID: payload.ChatID, SpaceID: payload.SpaceID, ActiveBranch: payload.TargetBranch}
			engine := thinkgraph.NewTraversalEngine(server.manager.GraphRepository())

			events, nextCursor, hasMore, err := engine.GetTimelinePage(ctx, chatObj, "", 50, topo)
			if err != nil {
				server.logger.Error("Failed getting timeline on switch branch", "error", err)
			} else if syncData, err := MarshalSyncHistory(events, nil, thinkgraph.AttentionFocus{}, nextCursor, hasMore); err == nil {
				ui.SendSyncHistory(syncData)
			}

		case "select_strategy":
			ui.PushStrategy(inbound.SelectStrategy.StrategyID)

		case "review_decision":
			ui.PushReviewDecision(inbound.ReviewDecision.Accepted)

		case "approve_context":
			decision := session.ContextApprovalDecision{
				Approved:               inbound.ApproveContext.Approved,
				RetainedFilePaths:      inbound.ApproveContext.RetainedFilePaths,
				RetainedHistoryIndices: inbound.ApproveContext.RetainedHistoryIndices,
			}
			ui.PushContextApproval(decision)

		case "transition_decision":
			ui.PushTransitionDecision(inbound.TransitionDecision.Approved)
		}
	}
}
