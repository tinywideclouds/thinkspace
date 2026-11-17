package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"uuid"

	"github.com/coder/websocket"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
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
	if logger == nil {
		logger = slog.Default()
	}
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

	if spaceID == "" || chatID == "" || previewID == "" {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	service := server.manager.GetService(spaceID)
	chatAddress := &thinkgraph.ChatAddress{
		ChatID:    chatID,
		ProjectID: spaceID,
	}

	data, err := service.GetContextPreview(r.Context(), chatAddress, previewID)
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

	chunkChan := make(chan workspace.NodeChunk, 100)
	connectionManager := NewConnectionManager(ctx, connection, chunkChan, server.logger)

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
		state, stateErr := server.manager.GetSpaceState(id)
		if stateErr == nil && state.ThinkState != "" {
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
		connectionManager.SendBytes(handshakeBytes)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case chunk := <-chunkChan:
				streamBytes, err := MarshalChatStream(chunk)
				if err != nil {
					server.logger.Error("Failed marshaling chat stream", "error", err)
					continue
				}
				connectionManager.SendBytes(streamBytes)
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
			if payload == nil || payload.Text == "" || payload.SpaceID == "" || payload.ChatID == "" {
				continue
			}

			emitter := NewConnectionEmitter(connectionManager)
			ui := NewTurnUI(connectionManager)

			go func(req SubmitPrompt) {
				server.logger.Info("Starting RunTurn goroutine", "space_id", req.SpaceID, "chat_id", req.ChatID, "prompt", req.Text)

				if err := server.turnManager.RunTurn(ctx, req.SpaceID, req.ChatID, req.Text, ui, emitter); err != nil {
					server.logger.Error("turn failed in execution", "space_id", req.SpaceID, "chat_id", req.ChatID, "error", err)
					emitter.Emit("system", telemetry.Event{
						ID:        uuid.NewV7(),
						Type:      "error",
						Timestamp: time.Now(),
						Message:   fmt.Sprintf("System Error: %v", err),
					})
				} else {
					server.logger.Info("Finished RunTurn goroutine successfully", "space_id", req.SpaceID, "chat_id", req.ChatID)
				}
			}(*payload)

		case "load_history":
			payload := inbound.LoadHistory
			if payload == nil || payload.SpaceID == "" || payload.ChatID == "" {
				continue
			}

			chatSession, err := server.turnManager.GetSession(ctx, payload.SpaceID, payload.ChatID)
			if err != nil {
				server.logger.Error("Failed retrieving session for history", "error", err)
				continue
			}

			chatAddress := chatSession.GetActiveAddress()

			service := server.manager.GetService(payload.SpaceID)
			if err := service.SyncGraphContext(ctx, chatAddress); err != nil {
				server.logger.Error("Failed JIT sync during load_history", "error", err)
			}

			limit := payload.Limit
			if limit <= 0 {
				limit = 50
			}

			engine := thinkgraph.NewTraversalEngine(server.manager.GraphRepository())
			events, nextCursor, hasMore, err := engine.GetTimelinePage(ctx, chatAddress, payload.Cursor, limit)
			if err != nil {
				server.logger.Error("Failed to fetch timeline page", "error", err)
				continue
			}

			var digests []thinkgraph.DigestMeta
			var focus thinkgraph.AttentionFocus

			if payload.Cursor == "" {
				if graphData, err := chatSession.GetGraphData(ctx); err == nil {
					digests = graphData.Digests
					focus = graphData.Focus
				} else {
					server.logger.Warn("Failed getting graph data for initial history load", "error", err)
				}
			}

			coordinate := ProjectionCoordinate{SpaceID: payload.SpaceID, ChatID: payload.ChatID, ProjectionID: chatAddress.Name}
			syncData, err := MarshalSyncHistory(events, digests, focus, nextCursor, hasMore, coordinate)
			if err != nil {
				server.logger.Error("Failed to marshal history sync payload", "error", err)
				continue
			}
			connectionManager.SendBytes(syncData)

		case "load_sub_graph":
			payload := inbound.LoadSubGraph
			if payload == nil || payload.SpaceID == "" || payload.ChatID == "" || payload.GraphID == "" {
				continue
			}

			var targetGraphID thinkgraph.GraphID
			if strings.HasPrefix(payload.GraphID, "flow-") {
				uuidStr := strings.TrimPrefix(payload.GraphID, "flow-")
				if parsedUUID, err := uuid.Parse(uuidStr); err == nil {
					targetGraphID = thinkgraph.GraphID{Prefix: thinkgraph.PrefixFlow, UUID: parsedUUID}
				} else {
					server.logger.Warn("Invalid UUID in flow projection ID", "graph_id", payload.GraphID, "error", err)
					continue
				}
			}

			chatAddress := &thinkgraph.ChatAddress{
				ChatID:    payload.ChatID,
				ProjectID: payload.SpaceID,
				SubGraph: thinkgraph.SubGraph{
					Name:    payload.GraphID,
					GraphID: targetGraphID,
				},
			}

			service := server.manager.GetService(payload.SpaceID)
			if err := service.SyncGraphContext(ctx, chatAddress); err != nil {
				server.logger.Error("Failed JIT sync during load_sub_graph", "error", err)
			}

			engine := thinkgraph.NewTraversalEngine(server.manager.GraphRepository())
			events, err := engine.GetFullTimeline(ctx, chatAddress)
			if err != nil {
				server.logger.Error("Failed to fetch subgraph timeline", "error", err)
				continue
			}

			coordinate := ProjectionCoordinate{SpaceID: payload.SpaceID, ChatID: payload.ChatID, ProjectionID: payload.GraphID}
			syncData, err := MarshalSyncSubGraph(events, coordinate)
			if err != nil {
				server.logger.Error("Failed to marshal sync subgraph payload", "error", err)
				continue
			}
			connectionManager.SendBytes(syncData)

		case "fork_chat":
			payload := inbound.ForkChat
			if payload == nil || payload.SpaceID == "" || payload.ChatID == "" {
				continue
			}

			chatSession, err := server.turnManager.GetSession(ctx, payload.SpaceID, payload.ChatID)
			if err != nil {
				server.logger.Error("Failed retrieving session for fork", "error", err)
				continue
			}

			targetID, err := uuid.Parse(payload.TargetEventID)
			if err != nil {
				server.logger.Error("Failed parsing target event ID for fork", "target_id", payload.TargetEventID, "error", err)
				continue
			}

			chatSession.Mu().Lock()
			parentBranch := chatSession.Meta.ActiveBranch
			id := uuid.NewV7()
			newBranchID := "branch-" + id.String()[:8]
			chatSession.Meta.ActiveBranch = newBranchID
			chatSession.Mu().Unlock()

			if err := server.manager.UpdateChatMeta(payload.SpaceID, payload.ChatID, func(m *workspace.ChatMeta) {
				m.ActiveBranch = newBranchID
			}); err != nil {
				server.logger.Error("Failed to update chat meta on fork", "error", err)
			}

			chatAddress := chatSession.GetActiveAddress()

			service := server.manager.GetService(payload.SpaceID)
			if err := service.AppendBranchAnchor(ctx, chatAddress, parentBranch, targetID); err != nil {
				server.logger.Error("Failed writing branch anchor", "error", err)
			}

			if err := service.SyncGraphContext(ctx, chatAddress); err != nil {
				server.logger.Error("Failed JIT sync during fork_chat", "error", err)
			}

			engine := thinkgraph.NewTraversalEngine(server.manager.GraphRepository())
			events, nextCursor, hasMore, err := engine.GetTimelinePage(ctx, chatAddress, "", 50)
			if err != nil {
				server.logger.Error("Failed to get timeline after fork", "error", err)
			} else {
				coordinate := ProjectionCoordinate{SpaceID: payload.SpaceID, ChatID: payload.ChatID, ProjectionID: newBranchID}
				syncData, marshalErr := MarshalSyncHistory(events, nil, thinkgraph.AttentionFocus{}, nextCursor, hasMore, coordinate)
				if marshalErr != nil {
					server.logger.Error("Failed to marshal fork sync history", "error", marshalErr)
				} else {
					connectionManager.SendBytes(syncData)
				}
			}

		case "inline_edit":
			payload := inbound.InlineEdit
			if payload == nil || payload.SpaceID == "" || payload.ChatID == "" {
				continue
			}

			chatSession, err := server.turnManager.GetSession(ctx, payload.SpaceID, payload.ChatID)
			if err != nil {
				server.logger.Error("Failed retrieving session for inline edit", "error", err)
				continue
			}

			chatAddress := chatSession.GetActiveAddress()

			if err := server.manager.GetService(payload.SpaceID).AppendUserPrompt(ctx, chatAddress, payload.NewText); err != nil {
				server.logger.Error("Failed logging amended prompt", "error", err)
			}

			if targetID, err := uuid.Parse(payload.TargetEventID); err != nil {
				server.logger.Error("Failed parsing target event ID for inline edit", "target_id", payload.TargetEventID, "error", err)
			} else {
				mut := thinkgraph.ContentMutation{
					BaseMutation: thinkgraph.BaseMutation{
						TargetID:  targetID,
						Type:      thinkgraph.MutationContent,
						Timestamp: time.Now().UTC(),
					},
					Content: payload.NewText,
				}
				if err := server.manager.GetService(payload.SpaceID).SaveMutation(ctx, chatAddress, mut); err != nil {
					server.logger.Error("Failed saving mutation for inline edit", "error", err)
				}
			}

		case "switch_branch":
			payload := inbound.SwitchBranch
			if payload == nil || payload.SpaceID == "" || payload.ChatID == "" || payload.TargetBranch == "" {
				continue
			}

			chatSession, err := server.turnManager.GetSession(ctx, payload.SpaceID, payload.ChatID)
			if err != nil {
				server.logger.Error("Failed retrieving session for switch branch", "error", err)
				continue
			}

			chatSession.Mu().Lock()
			chatSession.Meta.ActiveBranch = payload.TargetBranch
			chatSession.Mu().Unlock()

			if err := server.manager.UpdateChatMeta(payload.SpaceID, payload.ChatID, func(m *workspace.ChatMeta) {
				m.ActiveBranch = payload.TargetBranch
			}); err != nil {
				server.logger.Error("Failed updating meta on switch branch", "error", err)
			}

			chatAddress := chatSession.GetActiveAddress()

			service := server.manager.GetService(payload.SpaceID)
			if err := service.SyncGraphContext(ctx, chatAddress); err != nil {
				server.logger.Error("Failed JIT sync during switch_branch", "error", err)
			}

			engine := thinkgraph.NewTraversalEngine(server.manager.GraphRepository())

			events, nextCursor, hasMore, err := engine.GetTimelinePage(ctx, chatAddress, "", 50)
			if err != nil {
				server.logger.Error("Failed getting timeline on switch branch", "error", err)
			} else {
				coordinate := ProjectionCoordinate{SpaceID: payload.SpaceID, ChatID: payload.ChatID, ProjectionID: payload.TargetBranch}
				syncData, marshalErr := MarshalSyncHistory(events, nil, thinkgraph.AttentionFocus{}, nextCursor, hasMore, coordinate)
				if marshalErr != nil {
					server.logger.Error("Failed to marshal switch branch sync history", "error", marshalErr)
				} else {
					connectionManager.SendBytes(syncData)
				}
			}

		case "select_strategy":
			if inbound.SelectStrategy != nil {
				connectionManager.PushStrategy(inbound.SelectStrategy.StrategyID, inbound.SelectStrategy.AutoAccept)
			}

		case "review_decision":
			if inbound.ReviewDecision != nil {
				connectionManager.PushReviewDecision(inbound.ReviewDecision.Accepted, inbound.ReviewDecision.Reason)
			}

		case "approve_context":
			if inbound.ApproveContext != nil {
				decision := session.ContextApprovalDecision{
					Approved:               inbound.ApproveContext.Approved,
					RetainedFilePaths:      inbound.ApproveContext.RetainedFilePaths,
					RetainedHistoryIndices: inbound.ApproveContext.RetainedHistoryIndices,
				}
				connectionManager.PushContextApproval(decision)
			}

		case "transition_decision":
			if inbound.TransitionDecision != nil {
				connectionManager.PushTransitionDecision(inbound.TransitionDecision.Approved)
			}
		}
	}
}
