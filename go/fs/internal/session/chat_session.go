package session

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// ChatSession represents the active state and dependency container of a single conversation thread.
// It manages metadata locks, topology construction, and routes execution to the ChatEngine.
type ChatSession struct {
	mu      sync.RWMutex
	SpaceID string
	ChatID  string
	Meta    workspace.ChatMeta

	logger           *slog.Logger
	registry         *config.Registry
	workspaceManager *workspace.ServiceManager
	workspaceService *workspace.Service
	modelClient      llm.ModelClient
	bundleAssembler  *BundleAssembler
	digestManager    *DigestManager
	reviewPolicy     ContextReviewPolicy

	chatEngine *ChatEngine
}

func NewChatSession(
	ctx context.Context,
	logger *slog.Logger,
	registry *config.Registry,
	workspaceManager *workspace.ServiceManager,
	modelClient llm.ModelClient,
	fanOutFlow flows.Flow,
	digestManager *DigestManager,
	spaceID string,
	chatID string,
) (*ChatSession, error) {
	workspaceService := workspaceManager.GetService(spaceID)

	if _, err := workspaceService.InitializeChat(ctx, spaceID, chatID); err != nil {
		logger.ErrorContext(ctx, "failed initializing chat boundaries", "space_id", spaceID, "chat_id", chatID, "error", err)
		return nil, fmt.Errorf("initializing chat boundaries: %w", err)
	}

	meta, err := workspaceManager.GetChatMeta(spaceID, chatID)
	if err != nil {
		logger.ErrorContext(ctx, "failed loading chat meta", "space_id", spaceID, "chat_id", chatID, "error", err)
		return nil, fmt.Errorf("loading chat meta: %w", err)
	}

	if meta.ActiveThinkState == "" {
		if spaceState, stateErr := workspaceManager.GetSpaceState(spaceID); stateErr == nil && spaceState.ThinkState != "" {
			meta.ActiveThinkState = spaceState.ThinkState
		} else {
			meta.ActiveThinkState = "intake"
		}
		_ = workspaceManager.UpdateChatMeta(spaceID, chatID, func(m *workspace.ChatMeta) {
			m.ActiveThinkState = meta.ActiveThinkState
		})
	}

	assembler := NewBundleAssembler(
		logger,
		workspaceManager.GraphRepository(),
		thinkgraph.NewContextProjecter(),
		registry,
		workspaceService.WorkspaceRoot(),
	)

	turnExecutor := NewTurnExecutor(logger, modelClient, workspaceService, nil)

	session := &ChatSession{
		SpaceID:          spaceID,
		ChatID:           chatID,
		Meta:             meta,
		logger:           logger,
		registry:         registry,
		workspaceManager: workspaceManager,
		workspaceService: workspaceService,
		modelClient:      modelClient,
		bundleAssembler:  assembler,
		digestManager:    digestManager,
		reviewPolicy:     NewTokenThresholdPolicy(),
	}

	session.chatEngine = NewChatEngine(
		logger,
		session,
		registry,
		workspaceManager,
		workspaceService,
		assembler,
		digestManager,
		turnExecutor,
		fanOutFlow,
	)

	return session, nil
}

func (s *ChatSession) Mu() *sync.RWMutex {
	return &s.mu
}

func (s *ChatSession) SetReviewPolicy(policy ContextReviewPolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if policy == nil {
		policy = NewTokenThresholdPolicy()
	}
	s.reviewPolicy = policy
}

func (s *ChatSession) flushMeta() error {
	s.mu.RLock()
	activeThinkState := s.Meta.ActiveThinkState
	initializedThinkStates := s.Meta.InitializedThinkStates
	digestionHorizon := s.Meta.DigestionHorizon
	evaluationHorizon := s.Meta.EvaluationHorizon
	s.mu.RUnlock()

	return s.workspaceManager.UpdateChatMeta(s.SpaceID, s.ChatID, func(m *workspace.ChatMeta) {
		m.ActiveThinkState = activeThinkState
		m.InitializedThinkStates = initializedThinkStates
		m.DigestionHorizon = digestionHorizon
		m.EvaluationHorizon = evaluationHorizon
	})
}

// flushMetaLocked flushes metadata directly while a write lock is already held by the caller.
func (s *ChatSession) flushMetaLocked() error {
	activeThinkState := s.Meta.ActiveThinkState
	initializedThinkStates := s.Meta.InitializedThinkStates
	digestionHorizon := s.Meta.DigestionHorizon
	evaluationHorizon := s.Meta.EvaluationHorizon

	return s.workspaceManager.UpdateChatMeta(s.SpaceID, s.ChatID, func(m *workspace.ChatMeta) {
		m.ActiveThinkState = activeThinkState
		m.InitializedThinkStates = initializedThinkStates
		m.DigestionHorizon = digestionHorizon
		m.EvaluationHorizon = evaluationHorizon
	})
}

func (s *ChatSession) buildTopology() map[string]thinkgraph.BranchTopology {
	s.mu.RLock()
	defer s.mu.RUnlock()
	topo := make(map[string]thinkgraph.BranchTopology)
	for k, v := range s.Meta.Branches {
		topo[k] = thinkgraph.BranchTopology{ParentBranch: v.ParentBranch, ForkPoint: v.ForkPoint}
	}
	return topo
}

func (s *ChatSession) GetGraphData(ctx context.Context) (*thinkgraph.GraphData, error) {
	s.mu.RLock()
	activeBranch := s.Meta.ActiveBranch
	activeThinkState := s.Meta.ActiveThinkState
	digestionHorizon := s.Meta.DigestionHorizon
	s.mu.RUnlock()

	address := &thinkgraph.ChatAddress{
		ChatID:    s.ChatID,
		ProjectID: s.SpaceID,
		SubGraph: thinkgraph.SubGraph{
			Name: activeBranch,
		},
	}

	thinkState, ok := s.registry.GetThinkState(activeThinkState)
	if !ok {
		s.logger.WarnContext(ctx, "failed resolving think state for graph data", "thinkState", activeThinkState)
	}

	_, graphData, err := s.bundleAssembler.Assemble(ctx, address, s.buildTopology(), digestionHorizon, activeThinkState, thinkState)
	return graphData, err
}

func (s *ChatSession) RunTurn(ctx context.Context, promptText string, ui UserInterface, emitter telemetry.Emitter) error {
	return s.chatEngine.Run(ctx, promptText, ui, emitter)
}
