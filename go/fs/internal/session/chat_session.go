package session

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type ChatSession struct {
	mu      sync.RWMutex
	SpaceID string
	ChatID  string
	Meta    workspace.ChatMeta

	graphIDs map[string]thinkgraph.GraphID

	logger           *slog.Logger
	registry         *config.Registry
	workspaceManager *workspace.ServiceManager
	workspaceService *workspace.Service
	modelClient      llm.ModelClient
	bundleAssembler  *BundleAssembler
	digestManager    *DigestManager
	reviewPolicy     ContextReviewPolicy
	fanOutFlow       flows.Flow

	chatEngine *ChatEngine
	flowEngine *FlowEngine
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
	meta, err := workspaceManager.GetChatMeta(spaceID, chatID)
	if err != nil {
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

	workspaceService := workspaceManager.GetService(spaceID)

	session := &ChatSession{
		SpaceID:          spaceID,
		ChatID:           chatID,
		Meta:             meta,
		graphIDs:         make(map[string]thinkgraph.GraphID),
		logger:           logger,
		registry:         registry,
		workspaceManager: workspaceManager,
		workspaceService: workspaceService,
		modelClient:      modelClient,
		digestManager:    digestManager,
		reviewPolicy:     NewTokenThresholdPolicy(),
		fanOutFlow:       fanOutFlow,
	}

	address := session.getActiveAddress()
	if err := workspaceService.InitializeChat(ctx, address); err != nil {
		return nil, fmt.Errorf("initializing chat boundaries: %w", err)
	}

	assembler := NewBundleAssembler(
		logger,
		workspaceManager.GraphRepository(),
		thinkgraph.NewContextProjecter(),
		registry,
		workspaceService.WorkspaceRoot(),
	)

	turnExecutor := NewTurnExecutor(logger, modelClient, workspaceService, nil)

	session.bundleAssembler = assembler
	session.chatEngine = NewChatEngine(logger, session, registry, workspaceManager, workspaceService, assembler, digestManager, turnExecutor)
	session.flowEngine = NewFlowEngine(logger, workspaceService, turnExecutor, workspaceService.Dispatcher())

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

func (s *ChatSession) getActiveAddress() *thinkgraph.ChatAddress {
	s.mu.Lock()
	defer s.mu.Unlock()

	activeBranch := s.Meta.ActiveBranch
	if activeBranch == "" {
		activeBranch = "main"
	}

	gID, exists := s.graphIDs[activeBranch]
	if !exists {
		gID = thinkgraph.NewGraphID(thinkgraph.PrefixChat)
		s.graphIDs[activeBranch] = gID
	}

	return &thinkgraph.ChatAddress{
		ChatID:    s.ChatID,
		ProjectID: s.SpaceID,
		SubGraph: thinkgraph.SubGraph{
			Name:    activeBranch,
			GraphID: gID,
		},
	}
}

func (s *ChatSession) GetGraphData(ctx context.Context) (*thinkgraph.GraphData, error) {
	address := s.getActiveAddress()

	s.mu.RLock()
	activeThinkState := s.Meta.ActiveThinkState
	digestionHorizon := s.Meta.DigestionHorizon
	s.mu.RUnlock()

	thinkState, ok := s.registry.GetThinkState(activeThinkState)
	if !ok {
		s.logger.WarnContext(ctx, "failed resolving think state for graph data", "thinkState", activeThinkState)
	}

	_, graphData, err := s.bundleAssembler.Assemble(ctx, address, digestionHorizon, activeThinkState, thinkState)
	return graphData, err
}

func (s *ChatSession) RunTurn(ctx context.Context, promptText string, ui UserInterface, emitter telemetry.Emitter) error {
	address := s.getActiveAddress()

	var currentSignal signals.EngineSignal = &signals.SignalStartChat{}
	var err error

	for !currentSignal.IsTerminal() {
		switch sig := currentSignal.(type) {

		case *signals.SignalStartChat:
			currentSignal, err = s.chatEngine.Run(ctx, address, promptText, ui, emitter)
			promptText = ""
			if err != nil {
				return err
			}

		case *signals.SignalDispatchFlow:
			s.mu.RLock()
			activeThinkState := s.Meta.ActiveThinkState
			s.mu.RUnlock()

			thinkState, ok := s.registry.GetThinkState(activeThinkState)
			if !ok {
				return fmt.Errorf("think state not found: %s", activeThinkState)
			}

			currentSignal, err = s.flowEngine.Run(ctx, address, thinkState, s.fanOutFlow, sig.Tasks, sig.Summary, ui, emitter)
			if err != nil {
				return err
			}

		case *signals.SignalSynthesize:
			currentSignal, err = s.chatEngine.RunSynthesis(ctx, address, sig, ui, emitter)
			if err != nil {
				return err
			}

		case *signals.SignalTransition:
			if rebootErr := s.handleTransition(ctx, address, sig.TargetThinkState, ui, emitter); rebootErr != nil {
				return rebootErr
			}
			currentSignal = &signals.SignalStartChat{}

		default:
			return fmt.Errorf("supervisor encountered unknown engine signal: %T", sig)
		}
	}

	_, err = s.workspaceService.Checkpoint(ctx, address, "End of session turn")
	return err
}

func (s *ChatSession) handleTransition(ctx context.Context, address *thinkgraph.ChatAddress, targetThinkState string, ui UserInterface, emitter telemetry.Emitter) error {
	targetSpace, exists := s.registry.GetThinkState(targetThinkState)
	if !exists {
		ui.OnTextChunk(workspace.NodeChunk{NodeID: uuid.UUID{}, Text: fmt.Sprintf("\n\n⚠️ **System Error:** Think state '%s' does not exist in registry. Transition aborted.\n", targetThinkState)})
		return fmt.Errorf("invalid transition target: %s", targetThinkState)
	}

	chatDir := filepath.Join(s.workspaceService.WorkspaceRoot(), s.SpaceID, "chats", address.ChatID)

	s.mu.Lock()
	hasInitialized := false
	for _, d := range s.Meta.InitializedThinkStates {
		if d == targetThinkState {
			hasInitialized = true
			break
		}
	}

	if !hasInitialized {
		if initErr := targetSpace.Initialize(ctx, chatDir); initErr != nil {
			s.mu.Unlock()
			return fmt.Errorf("failed to initialize domain %s: %w", targetThinkState, initErr)
		}
		s.Meta.InitializedThinkStates = append(s.Meta.InitializedThinkStates, targetThinkState)
	}

	s.Meta.ActiveThinkState = targetThinkState
	if err := s.flushMetaLocked(); err != nil {
		s.logger.WarnContext(ctx, "failed to flush meta after domain transition", "error", err)
	}
	s.mu.Unlock()

	if emitter != nil {
		emitter.Emit("manager", telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "thinkstate_transition",
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("Workspace transitioned to %s domain.", targetThinkState),
			Payload:   map[string]any{"target_think_state": targetThinkState},
		})
	}

	if err := s.workspaceService.AppendDigest(ctx, address, fmt.Sprintf("Workspace transitioned to %s domain.", targetThinkState), []string{"system"}, false, nil); err != nil {
		s.logger.WarnContext(ctx, "failed appending transition digest", "error", err)
	}

	sysEv := thinkgraph.NewEvent(thinkgraph.EventSystem, fmt.Sprintf("Workspace transitioned to %s", targetThinkState))
	if err := s.workspaceService.Dispatcher().AppendEvent(ctx, address, sysEv); err != nil {
		s.logger.WarnContext(ctx, "failed appending transition system event", "error", err)
	}
	if ui != nil {
		ui.StreamGraphEvent(thinkgraph.Node{Event: sysEv})
	}

	return nil
}
