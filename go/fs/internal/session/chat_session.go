package session

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
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

const maxSessionLedgerLineBytes = 10 * 1024 * 1024

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
		spaceState, stateErr := workspaceManager.GetSpaceState(spaceID)
		if stateErr != nil && !errors.Is(stateErr, os.ErrNotExist) {
			return nil, fmt.Errorf("loading space state: %w", stateErr)
		}
		if stateErr == nil && spaceState.ThinkState != "" {
			meta.ActiveThinkState = spaceState.ThinkState
		} else {
			meta.ActiveThinkState = "intake"
		}
		if err := workspaceManager.UpdateChatMeta(spaceID, chatID, func(m *workspace.ChatMeta) {
			m.ActiveThinkState = meta.ActiveThinkState
		}); err != nil {
			return nil, fmt.Errorf("persisting initial chat think state: %w", err)
		}
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

	for branch, rawID := range meta.BranchGraphIDs {
		if strings.TrimSpace(rawID) == "" {
			continue
		}
		parsed, parseErr := thinkgraph.ParseGraphID(rawID)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing graph ID for branch %q: %w", branch, parseErr)
		}
		if !parsed.IsZero() {
			session.graphIDs[branch] = parsed
		}
	}

	address, err := session.resolveOrInitActiveAddress()
	if err != nil {
		return nil, fmt.Errorf("resolving active chat address: %w", err)
	}
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

	turnExecutor := NewTurnExecutor(logger, modelClient, workspaceService)

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
	branchGraphIDs := make(map[string]string, len(s.Meta.BranchGraphIDs))
	for k, v := range s.Meta.BranchGraphIDs {
		branchGraphIDs[k] = v
	}

	return s.workspaceManager.UpdateChatMeta(s.SpaceID, s.ChatID, func(m *workspace.ChatMeta) {
		m.ActiveThinkState = activeThinkState
		m.InitializedThinkStates = initializedThinkStates
		m.DigestionHorizon = digestionHorizon
		m.EvaluationHorizon = evaluationHorizon
		m.BranchGraphIDs = branchGraphIDs
	})
}

func (s *ChatSession) resolvePersistedGraphID(branch string) (thinkgraph.GraphID, bool, error) {
	if s.workspaceService == nil {
		return thinkgraph.GraphID{}, false, nil
	}

	dirName := thinkgraph.ResolveLogDirName(branch)
	logsDir := filepath.Join(s.workspaceService.WorkspaceRoot(), s.SpaceID, "chats", s.ChatID, "ledger", "logs", dirName)
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return thinkgraph.GraphID{}, false, nil
		}
		return thinkgraph.GraphID{}, false, fmt.Errorf("reading branch logs directory %s: %w", logsDir, err)
	}

	var shards []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			shards = append(shards, e.Name())
		}
	}
	if len(shards) == 0 {
		return thinkgraph.GraphID{}, false, nil
	}
	sort.Strings(shards)

	for _, shard := range shards {
		shardPath := filepath.Join(logsDir, shard)
		file, openErr := os.Open(shardPath)
		if openErr != nil {
			return thinkgraph.GraphID{}, false, fmt.Errorf("opening shard %s: %w", shardPath, openErr)
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64*1024), maxSessionLedgerLineBytes)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var ev thinkgraph.Event
			if unmarshalErr := json.Unmarshal(line, &ev); unmarshalErr != nil {
				_ = file.Close()
				return thinkgraph.GraphID{}, false, fmt.Errorf("unmarshaling event in %s at line %d: %w", shardPath, lineNum, unmarshalErr)
			}
			if !ev.GraphID.IsZero() {
				_ = file.Close()
				return ev.GraphID, true, nil
			}
		}

		scanErr := scanner.Err()
		_ = file.Close()
		if scanErr != nil {
			return thinkgraph.GraphID{}, false, fmt.Errorf("scanning shard %s: %w", shardPath, scanErr)
		}
	}

	return thinkgraph.GraphID{}, false, nil
}

func (s *ChatSession) resolveOrInitActiveAddress() (*thinkgraph.ChatAddress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	activeBranch := s.Meta.ActiveBranch
	if activeBranch == "" {
		activeBranch = "main"
	}

	gID, exists := s.graphIDs[activeBranch]
	if !exists {
		persistedID, ok, err := s.resolvePersistedGraphID(activeBranch)
		if err != nil {
			return nil, err
		}
		if ok {
			gID = persistedID
		} else {
			gID = thinkgraph.NewGraphID(thinkgraph.PrefixChat)
		}
		s.graphIDs[activeBranch] = gID
		if s.Meta.BranchGraphIDs == nil {
			s.Meta.BranchGraphIDs = make(map[string]string)
		}
		s.Meta.BranchGraphIDs[activeBranch] = gID.String()
		if err := s.flushMetaLocked(); err != nil {
			return nil, fmt.Errorf("persisting active branch graph ID: %w", err)
		}
	}

	return &thinkgraph.ChatAddress{
		ChatID:    s.ChatID,
		ProjectID: s.SpaceID,
		SubGraph: thinkgraph.SubGraph{
			Name:    activeBranch,
			GraphID: gID,
		},
	}, nil
}

// GetActiveAddress returns the canonical ChatAddress (including its resolved GraphID)
// for the session's currently active branch.
func (s *ChatSession) GetActiveAddress() *thinkgraph.ChatAddress {
	addr, err := s.resolveOrInitActiveAddress()
	if err != nil && s.logger != nil {
		s.logger.Error("failed resolving active chat address", "chat_id", s.ChatID, "error", err)
	}
	if addr != nil {
		return addr
	}

	s.mu.RLock()
	activeBranch := s.Meta.ActiveBranch
	if activeBranch == "" {
		activeBranch = "main"
	}
	gID := s.graphIDs[activeBranch]
	s.mu.RUnlock()

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
		persistedID, ok, err := s.resolvePersistedGraphID(thinkgraph.SystemSubGraphName)
		if err != nil && s.logger != nil {
			s.logger.Error("failed resolving persisted system graph ID", "chat_id", s.ChatID, "error", err)
		}
		if ok {
			gID = persistedID
		} else {
			gID = thinkgraph.NewGraphID(thinkgraph.PrefixSystem)
		}
		s.graphIDs[thinkgraph.SystemSubGraphName] = gID
		if s.Meta.BranchGraphIDs == nil {
			s.Meta.BranchGraphIDs = make(map[string]string)
		}
		s.Meta.BranchGraphIDs[thinkgraph.SystemSubGraphName] = gID.String()
		if err := s.flushMetaLocked(); err != nil && s.logger != nil {
			s.logger.Error("failed to persist system subgraph ID", "graph_id", gID.String(), "error", err)
		}
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
	address, err := s.resolveOrInitActiveAddress()
	if err != nil {
		return nil, fmt.Errorf("resolving active address for graph data: %w", err)
	}

	s.mu.RLock()
	activeThinkState := s.Meta.ActiveThinkState
	digestionHorizon := s.Meta.DigestionHorizon
	s.mu.RUnlock()

	thinkState, ok := s.registry.GetThinkState(activeThinkState)
	if !ok && s.logger != nil {
		s.logger.WarnContext(ctx, "failed resolving think state for graph data", "thinkState", activeThinkState)
	}

	_, graphData, err := s.bundleAssembler.Assemble(ctx, address, digestionHorizon, uuid.Nil(), activeThinkState, thinkState)
	return graphData, err
}

func (s *ChatSession) RunTurn(ctx context.Context, promptText string, ui UserInterface, emitter telemetry.Emitter) error {
	address, err := s.resolveOrInitActiveAddress()
	if err != nil {
		return fmt.Errorf("resolving active address for turn: %w", err)
	}

	if err := s.workspaceService.AppendUserPrompt(ctx, address, promptText); err != nil {
		if s.logger != nil {
			s.logger.ErrorContext(ctx, "failed appending user prompt to ledger", "chat_id", s.ChatID, "error", err)
		}
		return fmt.Errorf("appending user prompt: %w", err)
	}

	traversalEngine := thinkgraph.NewTraversalEngine(s.workspaceManager.GraphRepository())
	timeline, _, _, err := traversalEngine.GetTimelinePage(ctx, address, "", 1)
	if err != nil {
		return fmt.Errorf("querying catalyst prompt from graph: %w", err)
	}
	if len(timeline) == 0 {
		return fmt.Errorf("catalyst prompt not found on graph timeline after append")
	}

	if ui != nil {
		ui.StreamGraphEvent(address, timeline[0])
	}
	catalystID := timeline[0].ID

	turnID := uuid.NewV7()
	var currentSignal signals.EngineSignal = &signals.SignalExecuteChat{}
	var activeFlow signals.FlowHandle

	defer func() {
		if activeFlow != nil {
			if finalizeErr := activeFlow.Finalize(ctx, "Turn terminated"); finalizeErr != nil && s.logger != nil {
				s.logger.ErrorContext(ctx, "failed to finalize active flow scope on turn exit", "error", finalizeErr)
			}
		}
	}()

	for !currentSignal.IsTerminal() {
		var engine TurnEngine

		switch sig := currentSignal.(type) {
		case *signals.SignalExecuteChat:
			engine = s.chatEngine
		case *signals.SignalDispatchFlow:
			engine = s.workerEngine
		case *signals.SignalSynthesize:
			activeFlow = sig.Flow
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
