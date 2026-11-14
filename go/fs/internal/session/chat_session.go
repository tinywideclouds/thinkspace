package session

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

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

	chatEngine       *ChatEngine
	workerEngine     *WorkerEngine
	systemEngine     *SystemEngine
	synthesizeEngine *SynthesizeEngine
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
	}

	address := session.GetActiveAddress()
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
	session.bundleAssembler = assembler

	turnExecutor := NewTurnExecutor(logger, modelClient, workspaceService, nil)

	session.chatEngine = NewChatEngine(logger, session, registry, workspaceManager, workspaceService, assembler, digestManager, turnExecutor)
	session.workerEngine = NewWorkerEngine(logger, session, registry, workspaceService, turnExecutor, workspaceService.Dispatcher(), fanOutFlow)
	session.systemEngine = NewSystemEngine(logger, session, registry, workspaceService)
	session.synthesizeEngine = NewSynthesizeEngine(logger, registry, workspaceService, turnExecutor, session)

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

func (s *ChatSession) resolvePersistedGraphID(branch string) (thinkgraph.GraphID, bool) {
	if s.workspaceService == nil {
		return thinkgraph.GraphID{}, false
	}

	dirName := thinkgraph.ResolveLogDirName(branch)
	logsDir := filepath.Join(s.workspaceService.WorkspaceRoot(), s.SpaceID, "chats", s.ChatID, "ledger", "logs", dirName)
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		return thinkgraph.GraphID{}, false
	}

	var shards []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			shards = append(shards, e.Name())
		}
	}
	if len(shards) == 0 {
		return thinkgraph.GraphID{}, false
	}
	sort.Strings(shards)

	for _, shard := range shards {
		file, err := os.Open(filepath.Join(logsDir, shard))
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var ev thinkgraph.Event
			if json.Unmarshal(line, &ev) == nil && !ev.GraphID.IsZero() {
				file.Close()
				return ev.GraphID, true
			}
		}

		scanErr := scanner.Err()
		if scanErr != nil {
			slog.Debug("scanner fault", "err", scanErr)
			continue
		}

		file.Close()

	}

	return thinkgraph.GraphID{}, false
}

// GetActiveAddress returns the canonical ChatAddress (including its resolved GraphID)
// for the session's currently active branch.
func (s *ChatSession) GetActiveAddress() *thinkgraph.ChatAddress {
	s.mu.Lock()
	defer s.mu.Unlock()

	activeBranch := s.Meta.ActiveBranch
	if activeBranch == "" {
		activeBranch = "main"
	}

	gID, exists := s.graphIDs[activeBranch]
	if !exists {
		if persistedID, ok := s.resolvePersistedGraphID(activeBranch); ok {
			gID = persistedID
		} else {
			gID = thinkgraph.NewGraphID(thinkgraph.PrefixChat)
		}
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

func (s *ChatSession) GetSystemAddress() *thinkgraph.ChatAddress {
	s.mu.Lock()
	defer s.mu.Unlock()

	gID, exists := s.graphIDs[thinkgraph.SystemSubGraphName]
	if !exists {
		if persistedID, ok := s.resolvePersistedGraphID(thinkgraph.SystemSubGraphName); ok {
			gID = persistedID
		} else {
			gID = thinkgraph.NewGraphID(thinkgraph.PrefixSystem)
		}
		s.graphIDs[thinkgraph.SystemSubGraphName] = gID
	}

	return &thinkgraph.ChatAddress{
		ChatID:    s.ChatID,
		ProjectID: s.SpaceID,
		SubGraph: thinkgraph.SubGraph{
			Name:    thinkgraph.SystemSubGraphName,
			GraphID: gID,
		},
	}
}

func (s *ChatSession) GetGraphData(ctx context.Context) (*thinkgraph.GraphData, error) {
	address := s.GetActiveAddress()

	s.mu.RLock()
	activeThinkState := s.Meta.ActiveThinkState
	digestionHorizon := s.Meta.DigestionHorizon
	s.mu.RUnlock()

	thinkState, ok := s.registry.GetThinkState(activeThinkState)
	if !ok {
		s.logger.WarnContext(ctx, "failed resolving think state for graph data", "thinkState", activeThinkState)
	}

	_, graphData, err := s.bundleAssembler.Assemble(ctx, address, digestionHorizon, uuid.Nil(), activeThinkState, thinkState)
	return graphData, err
}

func (s *ChatSession) RunTurn(ctx context.Context, promptText string, ui UserInterface, emitter telemetry.Emitter) error {
	address := s.GetActiveAddress()

	if err := s.workspaceService.AppendUserPrompt(ctx, address, promptText); err != nil {
		s.logger.ErrorContext(ctx, "failed appending user prompt to ledger", "chat_id", s.ChatID, "error", err)
		return fmt.Errorf("appending user prompt: %w", err)
	}

	traversalEngine := thinkgraph.NewTraversalEngine(s.workspaceManager.GraphRepository())
	timeline, _, _, err := traversalEngine.GetTimelinePage(ctx, address, "", 1)

	var catalystID uuid.UUID
	if err == nil && len(timeline) > 0 {
		ui.StreamGraphEvent(address, timeline[0])
		catalystID = timeline[0].ID
	} else {
		return fmt.Errorf("failed to anchor catalyst prompt to graph: %w", err)
	}

	turnID := uuid.NewV7()
	var currentSignal signals.EngineSignal = &signals.SignalExecuteChat{}

	for !currentSignal.IsTerminal() {
		var engine TurnEngine

		switch currentSignal.(type) {
		case *signals.SignalExecuteChat:
			engine = s.chatEngine
		case *signals.SignalDispatchFlow:
			engine = s.workerEngine
		case *signals.SignalSynthesize:
			engine = s.synthesizeEngine
		case *signals.SignalTransition:
			engine = s.systemEngine
		default:
			return fmt.Errorf("supervisor encountered unknown engine signal: %T", currentSignal)
		}

		currentSignal, err = engine.Execute(ctx, address, turnID, catalystID, currentSignal, ui, emitter)
		if err != nil {
			return err
		}
	}

	_, err = s.workspaceService.Checkpoint(ctx, address, "End of session turn")
	return err
}
