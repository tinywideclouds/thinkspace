package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"uuid"
)

type Server struct {
	logger        *slog.Logger
	registry      *config.Registry
	manager       *workspace.ServiceManager
	fanOutFlow    flows.Flow
	digestManager *session.DigestManager
	modelClient   llm.ModelClient

	sessionMu     sync.Mutex
	activeSession *session.ChatSession
}

func NewServer(
	logger *slog.Logger,
	registry *config.Registry,
	manager *workspace.ServiceManager,
	fanOutFlow flows.Flow,
	digestManager *session.DigestManager,
	modelClient llm.ModelClient,
) *Server {
	return &Server{
		logger:        logger,
		registry:      registry,
		manager:       manager,
		fanOutFlow:    fanOutFlow,
		digestManager: digestManager,
		modelClient:   modelClient,
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
	_, _ = w.Write(data)
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
	ui := NewWebSocketUI(ctx, connection, tokenChan)

	spaceIDs, _ := server.manager.ListSpaces(ctx)
	var availableSpaces []SpaceState

	for _, id := range spaceIDs {
		if id == "configs" {
			continue
		}

		isConfigured := false
		state, err := server.manager.GetSpaceState(id)

		if err == nil && state.Domain != "" {
			if _, ok := server.registry.GetDomain(state.Domain); ok {
				isConfigured = true
			}
		}

		availableSpaces = append(availableSpaces, SpaceState{
			ID:           id,
			Name:         id,
			IsConfigured: isConfigured,
		})
	}

	if handshakeBytes, err := MarshalAvailableSpaces(availableSpaces); err == nil {
		_ = connection.Write(ctx, websocket.MessageText, handshakeBytes)
	}

	go func() {
		for token := range tokenChan {
			if streamBytes, err := MarshalAgentStream(token.AgentID, token.Text); err == nil {
				_ = connection.Write(ctx, websocket.MessageText, streamBytes)
			}
		}
	}()

	for {
		_, msgBytes, err := connection.Read(ctx)
		if err != nil {
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

			server.sessionMu.Lock()
			if server.activeSession == nil || server.activeSession.ChatID != payload.ChatID {
				newSession, err := session.OpenChat(ctx, server.logger, server.registry, server.manager, server.modelClient, server.fanOutFlow, server.digestManager, payload.SpaceID, payload.ChatID)
				if err != nil {
					server.sessionMu.Unlock()
					server.logger.Error("failed to open chat session", "error", err)
					continue
				}
				server.activeSession = newSession

				graphData, err := server.activeSession.GetGraphData(ctx)
				if err == nil {
					if syncData, err := MarshalSyncHistory(graphData.Timeline, graphData.Digests, graphData.Focus, "", false); err == nil {
						ui.SendSyncHistory(syncData)
					}
				}
			}
			activeSession := server.activeSession
			server.sessionMu.Unlock()

			if payload.Text == "" {
				continue
			}

			emitter := NewWebSocketEmitter(ctx, connection)
			go func() {
				if err := activeSession.RunTurn(ctx, payload.Text, ui, emitter); err != nil {
					server.logger.Error("turn failed", "error", err)
					ui.OnTextChunk(fmt.Sprintf("\n\n⚠️ System Error: %v\n", err))
				}
			}()

		case "load_history":
			payload := inbound.LoadHistory

			server.sessionMu.Lock()
			activeBranch := "main"
			if server.activeSession != nil && server.activeSession.ChatID == payload.ChatID {
				activeBranch = server.activeSession.Meta.ActiveBranch
			}
			server.sessionMu.Unlock()

			chatObj := &thinkgraph.Chat{ID: payload.ChatID, SpaceID: payload.SpaceID, ActiveBranch: activeBranch}

			limit := payload.Limit
			if limit <= 0 {
				limit = 50
			}

			engine := thinkgraph.NewTraversalEngine(server.manager.GraphRepository())
			topo := make(map[string]thinkgraph.BranchTopology)
			server.sessionMu.Lock()
			if server.activeSession != nil {
				for k, v := range server.activeSession.Meta.Branches {
					topo[k] = thinkgraph.BranchTopology{ParentBranch: v.ParentBranch, ForkPoint: v.ForkPoint}
				}
			}
			server.sessionMu.Unlock()

			events, nextCursor, hasMore, err := engine.GetTimelinePage(ctx, chatObj, payload.Cursor, limit, topo)
			if err != nil {
				server.logger.Error("Failed to fetch timeline page", "error", err)
				continue
			}

			if syncData, err := MarshalSyncHistory(events, nil, thinkgraph.AttentionFocus{}, nextCursor, hasMore); err == nil {
				ui.SendSyncHistory(syncData)
			}

		case "fork_chat":
			payload := inbound.ForkChat
			server.sessionMu.Lock()
			if server.activeSession != nil && server.activeSession.ChatID == payload.ChatID {
				id := uuid.NewV7()
				newBranchID := "branch-" + id.String()[:8]

				server.activeSession.Meta.Branches[newBranchID] = workspace.BranchMeta{
					ParentBranch: server.activeSession.Meta.ActiveBranch,
					ForkPoint:    payload.TargetEventID,
				}
				server.activeSession.Meta.ActiveBranch = newBranchID

				_ = server.manager.UpdateChatMeta(payload.SpaceID, payload.ChatID, func(m *workspace.ChatMeta) {
					m.Branches[newBranchID] = server.activeSession.Meta.Branches[newBranchID]
					m.ActiveBranch = newBranchID
				})

				chatObj := &thinkgraph.Chat{ID: payload.ChatID, SpaceID: payload.SpaceID, ActiveBranch: newBranchID}
				engine := thinkgraph.NewTraversalEngine(server.manager.GraphRepository())
				topo := make(map[string]thinkgraph.BranchTopology)
				for k, v := range server.activeSession.Meta.Branches {
					topo[k] = thinkgraph.BranchTopology{ParentBranch: v.ParentBranch, ForkPoint: v.ForkPoint}
				}

				events, nextCursor, hasMore, _ := engine.GetTimelinePage(ctx, chatObj, "", 50, topo)
				if syncData, err := MarshalSyncHistory(events, nil, thinkgraph.AttentionFocus{}, nextCursor, hasMore); err == nil {
					ui.SendSyncHistory(syncData)
				}
			}
			server.sessionMu.Unlock()

		case "inline_edit":
			payload := inbound.InlineEdit
			server.sessionMu.Lock()
			if server.activeSession != nil && server.activeSession.ChatID == payload.ChatID {
				chatObj := &thinkgraph.Chat{ID: payload.ChatID, SpaceID: payload.SpaceID, ActiveBranch: server.activeSession.Meta.ActiveBranch}

				newEvent := thinkgraph.NewEvent(thinkgraph.EventPrompt, payload.NewText)
				newEvent.Metadata["amends"] = payload.TargetEventID
				_ = server.manager.GetService(payload.SpaceID).LogUserPrompt(ctx, chatObj, payload.NewText)

				if targetUUID, err := uuid.Parse(payload.TargetEventID); err == nil {
					mut := thinkgraph.MutationEvent{
						TargetID:  targetUUID,
						Type:      thinkgraph.MutationUpdate,
						Timestamp: time.Now().UTC(),
						Payload:   payload.NewText,
					}
					_ = server.manager.GetService(payload.SpaceID).SaveMutation(ctx, chatObj, mut)
				}
			}
			server.sessionMu.Unlock()

		case "switch_branch":
			payload := inbound.SwitchBranch
			server.sessionMu.Lock()
			if server.activeSession != nil && server.activeSession.ChatID == payload.ChatID {
				server.activeSession.Meta.ActiveBranch = payload.TargetBranch
				_ = server.manager.UpdateChatMeta(payload.SpaceID, payload.ChatID, func(m *workspace.ChatMeta) {
					m.ActiveBranch = payload.TargetBranch
				})

				chatObj := &thinkgraph.Chat{ID: payload.ChatID, SpaceID: payload.SpaceID, ActiveBranch: payload.TargetBranch}
				engine := thinkgraph.NewTraversalEngine(server.manager.GraphRepository())
				topo := make(map[string]thinkgraph.BranchTopology)
				for k, v := range server.activeSession.Meta.Branches {
					topo[k] = thinkgraph.BranchTopology{ParentBranch: v.ParentBranch, ForkPoint: v.ForkPoint}
				}

				events, nextCursor, hasMore, _ := engine.GetTimelinePage(ctx, chatObj, "", 50, topo)
				if syncData, err := MarshalSyncHistory(events, nil, thinkgraph.AttentionFocus{}, nextCursor, hasMore); err == nil {
					ui.SendSyncHistory(syncData)
				}
			}
			server.sessionMu.Unlock()

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
