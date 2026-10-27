package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
	"uuid"

	"github.com/coder/websocket"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type Server struct {
	logger        *slog.Logger
	registry      *config.Registry
	manager       *workspace.ServiceManager
	fanOutFlow    flows.Flow
	digestManager *session.DigestManager
	modelClient   llm.ModelClient

	// Single-user local architecture state
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

func (s *Server) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/receipts/{chat_id}/{flow_id}", s.handleGetReceipt)
	mux.HandleFunc("GET /api/previews/{chat_id}/{preview_id}", s.handleGetPreview)
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
	chatSession := &thinkgraph.Chat{
		ID:      chatID,
		SpaceID: spaceID,
	}

	data, err := service.GetReceipt(r.Context(), chatSession, flowID)
	if err != nil {
		http.Error(w, "Receipt not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	_, _ = w.Write(data)
}

func (s *Server) handleGetPreview(w http.ResponseWriter, r *http.Request) {
	chatID := r.PathValue("chat_id")
	previewID := r.PathValue("preview_id")
	spaceID := r.URL.Query().Get("space")

	if spaceID == "" {
		http.Error(w, "Missing space query parameter", http.StatusBadRequest)
		return
	}

	service := s.manager.GetService(spaceID)
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

	tokenChan := make(chan workspace.AgentToken, 100)
	ui := NewWebSocketUI(ctx, connection, tokenChan)

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
			s.logger.Warn("invalid websocket message", "error", err)
			continue
		}

		switch inbound.Type {
		case "submit_prompt":
			payload := inbound.SubmitPrompt

			s.sessionMu.Lock()
			if s.activeSession == nil || s.activeSession.ChatID != payload.ChatID {
				newSession, err := session.OpenChat(ctx, s.logger, s.registry, s.manager, s.modelClient, s.fanOutFlow, s.digestManager, payload.SpaceID, payload.ChatID)
				if err != nil {
					s.sessionMu.Unlock()
					s.logger.Error("failed to open chat session", "error", err)
					continue
				}
				s.activeSession = newSession

				graphData, err := s.activeSession.GetGraphData(ctx)
				if err == nil {
					if syncData, err := MarshalSyncHistory(graphData.Timeline, graphData.Digests, graphData.Focus, "", false); err == nil {
						ui.SendSyncHistory(syncData)
					}
				}
			}
			activeSession := s.activeSession
			s.sessionMu.Unlock()

			if payload.Text == "" {
				continue
			}

			emitter := NewWebSocketEmitter(ctx, connection)
			go func() {
				if err := activeSession.RunTurn(ctx, payload.Text, ui, emitter); err != nil {
					s.logger.Error("turn failed", "error", err)
					ui.OnTextChunk(fmt.Sprintf("\n\n⚠️ System Error: %v\n", err))
				}
			}()

		case "load_history":
			payload := inbound.LoadHistory

			s.sessionMu.Lock()
			activeBranch := "main"
			if s.activeSession != nil && s.activeSession.ChatID == payload.ChatID {
				activeBranch = s.activeSession.Meta.ActiveBranch
			}
			s.sessionMu.Unlock()

			chatObj := &thinkgraph.Chat{ID: payload.ChatID, SpaceID: payload.SpaceID, ActiveBranch: activeBranch}

			limit := payload.Limit
			if limit <= 0 {
				limit = 50
			}

			// Use the Traversal Engine to allow seamless cross-branch pagination
			engine := thinkgraph.NewTraversalEngine(s.manager.GraphStore())
			topo := make(map[string]thinkgraph.BranchTopology)
			s.sessionMu.Lock()
			if s.activeSession != nil {
				for k, v := range s.activeSession.Meta.Branches {
					topo[k] = thinkgraph.BranchTopology{ParentBranch: v.ParentBranch, ForkPoint: v.ForkPoint}
				}
			}
			s.sessionMu.Unlock()

			events, nextCursor, hasMore, err := engine.GetTimelinePage(ctx, chatObj, payload.Cursor, limit, topo)
			if err != nil {
				s.logger.Error("Failed to fetch timeline page", "error", err)
				continue
			}

			if syncData, err := MarshalSyncHistory(events, nil, thinkgraph.AttentionFocus{}, nextCursor, hasMore); err == nil {
				ui.SendSyncHistory(syncData)
			}

		case "fork_chat":
			payload := inbound.ForkChat
			s.sessionMu.Lock()
			if s.activeSession != nil && s.activeSession.ChatID == payload.ChatID {
				newBranchID := "branch-" + uuid.New().String()[:8]

				s.activeSession.Meta.Branches[newBranchID] = workspace.BranchMeta{
					ParentBranch: s.activeSession.Meta.ActiveBranch,
					ForkPoint:    payload.TargetEventID,
				}
				s.activeSession.Meta.ActiveBranch = newBranchID

				_ = s.manager.UpdateChatMeta(payload.SpaceID, payload.ChatID, func(m *workspace.ChatMeta) {
					m.Branches[newBranchID] = s.activeSession.Meta.Branches[newBranchID]
					m.ActiveBranch = newBranchID
				})

				chatObj := &thinkgraph.Chat{ID: payload.ChatID, SpaceID: payload.SpaceID, ActiveBranch: newBranchID}
				engine := thinkgraph.NewTraversalEngine(s.manager.GraphStore())
				topo := make(map[string]thinkgraph.BranchTopology)
				for k, v := range s.activeSession.Meta.Branches {
					topo[k] = thinkgraph.BranchTopology{ParentBranch: v.ParentBranch, ForkPoint: v.ForkPoint}
				}

				events, nextCursor, hasMore, _ := engine.GetTimelinePage(ctx, chatObj, "", 50, topo)
				if syncData, err := MarshalSyncHistory(events, nil, thinkgraph.AttentionFocus{}, nextCursor, hasMore); err == nil {
					ui.SendSyncHistory(syncData)
				}
			}
			s.sessionMu.Unlock()

		case "inline_edit":
			payload := inbound.InlineEdit
			s.sessionMu.Lock()
			if s.activeSession != nil && s.activeSession.ChatID == payload.ChatID {
				chatObj := &thinkgraph.Chat{ID: payload.ChatID, SpaceID: payload.SpaceID, ActiveBranch: s.activeSession.Meta.ActiveBranch}

				newEvent := thinkgraph.NewEvent(thinkgraph.EventPrompt, payload.NewText)
				newEvent.Metadata["amends"] = payload.TargetEventID
				_ = s.manager.GetService(payload.SpaceID).LogUserMessage(ctx, chatObj, payload.NewText) // Using LogUserMessage ensures it goes through the bus to both Badger and Ledger

				// Apply mutation to old event to create the forward link
				if targetUUID, err := uuid.Parse(payload.TargetEventID); err == nil {
					mut := thinkgraph.MutationEvent{
						TargetID:  targetUUID,
						Type:      thinkgraph.MutationUpdate, // Handled implicitly as an update by the UI/Rehydrator depending on implementation payload semantics
						Timestamp: time.Now().UTC(),
						Payload:   payload.NewText,
					}
					_ = s.manager.GetService(payload.SpaceID).SaveMutation(ctx, chatObj, mut)
				}
			}
			s.sessionMu.Unlock()

		case "switch_branch":
			payload := inbound.SwitchBranch
			s.sessionMu.Lock()
			if s.activeSession != nil && s.activeSession.ChatID == payload.ChatID {
				s.activeSession.Meta.ActiveBranch = payload.TargetBranch
				_ = s.manager.UpdateChatMeta(payload.SpaceID, payload.ChatID, func(m *workspace.ChatMeta) {
					m.ActiveBranch = payload.TargetBranch
				})

				chatObj := &thinkgraph.Chat{ID: payload.ChatID, SpaceID: payload.SpaceID, ActiveBranch: payload.TargetBranch}
				engine := thinkgraph.NewTraversalEngine(s.manager.GraphStore())
				topo := make(map[string]thinkgraph.BranchTopology)
				for k, v := range s.activeSession.Meta.Branches {
					topo[k] = thinkgraph.BranchTopology{ParentBranch: v.ParentBranch, ForkPoint: v.ForkPoint}
				}

				events, nextCursor, hasMore, _ := engine.GetTimelinePage(ctx, chatObj, "", 50, topo)
				if syncData, err := MarshalSyncHistory(events, nil, thinkgraph.AttentionFocus{}, nextCursor, hasMore); err == nil {
					ui.SendSyncHistory(syncData)
				}
			}
			s.sessionMu.Unlock()

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
