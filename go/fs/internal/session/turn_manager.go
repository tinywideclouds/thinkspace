package session

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// TurnManager orchestrates the lifecycle of a conversational turn entirely decoupled from network transport.
type TurnManager struct {
	logger           *slog.Logger
	registry         *config.Registry
	workspaceManager *workspace.ServiceManager
	llmAdapter       *llm.Adapter
	playbackEngine   *chat.PlaybackEngine
	contextAssembler *ContextAssembler
	fanOutFlow       flows.Flow
	modelClient      llm.ModelClient
}

func NewTurnManager(
	logger *slog.Logger,
	registry *config.Registry,
	workspaceManager *workspace.ServiceManager,
	llmAdapter *llm.Adapter,
	fanOutFlow flows.Flow,
	modelClient llm.ModelClient,
) *TurnManager {
	return &TurnManager{
		logger:           logger,
		registry:         registry,
		workspaceManager: workspaceManager,
		llmAdapter:       llmAdapter,
		playbackEngine:   chat.NewPlaybackEngine(),
		contextAssembler: NewContextAssembler(),
		fanOutFlow:       fanOutFlow,
		modelClient:      modelClient,
	}
}

// LoadThreadState provisions a thread and loads its history. Ideal for initial UI hydration.
func (tm *TurnManager) LoadThreadState(ctx context.Context, spaceID, chatID string) (*chat.Thread, *chat.ChatGraph, *chat.ThreadManifest, error) {
	workspaceService := tm.workspaceManager.GetService(spaceID)
	thread, err := workspaceService.StartThread(ctx, chatID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("starting thread: %w", err)
	}

	graph, manifest, err := tm.playbackEngine.LoadState(ctx, thread)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("loading state: %w", err)
	}

	return thread, graph, manifest, nil
}

// ExecuteTurn handles the guaranteed sequence of logging a prompt, reloading history, and invoking the LLM.
func (tm *TurnManager) ExecuteTurn(ctx context.Context, spaceID, chatID, promptText string, ui UserInterface, emitter flows.FlowEmitter) error {
	spaceState, err := tm.workspaceManager.GetSpaceState(spaceID)
	if err != nil {
		return fmt.Errorf("getting space state: %w", err)
	}

	thinkSpace, ok := tm.registry.GetDomain(spaceState.Domain)
	if !ok {
		return fmt.Errorf("domain configuration not found: %s", spaceState.Domain)
	}

	spaceConfig, ok := tm.registry.GetConfig(spaceState.Domain)
	if !ok {
		return fmt.Errorf("space configuration not found: %s", spaceState.Domain)
	}

	flowConfig, ok := tm.registry.GetFlow("fanout")
	if !ok {
		return fmt.Errorf("fanout flow configuration missing")
	}

	workspaceService := tm.workspaceManager.GetService(spaceID)
	thread, err := workspaceService.StartThread(ctx, chatID)
	if err != nil {
		return fmt.Errorf("starting thread: %w", err)
	}

	// 1. ALWAYS log the user's prompt to the physical ledger first
	if promptText != "" {
		if err := workspaceService.LogUserPrompt(ctx, thread, promptText); err != nil {
			return fmt.Errorf("logging user prompt: %w", err)
		}
	}

	// 2. Reload state to guarantee the newly written prompt is included in the RecentEvents array
	graph, manifest, err := tm.playbackEngine.LoadState(ctx, thread)
	if err != nil {
		return fmt.Errorf("loading updated state: %w", err)
	}

	// 3. Assemble and fuse the context dual-brain
	assemblyReq := chat.AssemblyRequest{
		Manifest:     manifest,
		ActiveLenses: []string{},
		RecentEvents: graph.RecentEvents,
	}

	dynamicSystemPrompt, history, err := tm.contextAssembler.Build(ctx, thinkSpace, workspaceService.WorkspaceRoot(), assemblyReq)
	if err != nil {
		return fmt.Errorf("assembling context: %w", err)
	}

	workerModel := thinkSpace.Config().Models[spaces.ModelCategoryWorker]
	executor := llm.NewSubAgentExecutor(tm.modelClient, workerModel, thinkSpace.Config().WorkerSystemPrompt(), thinkSpace.Config().MaxWorkerTokens)

	coordinator := NewCoordinator(
		tm.logger,
		workspaceService,
		tm.llmAdapter,
		executor,
		tm.fanOutFlow,
		emitter,
		spaceID,
		spaceConfig.Roles.Worker,
		flowConfig,
	)

	// --- Observability: Manager LLM Payload ---
	tm.logger.Info("=== MANAGER CONTEXT ASSEMBLY COMPLETE ===",
		"system_prompt_length", len(dynamicSystemPrompt),
		"history_length", len(history),
	)

	// Use Debug level for the massive text dumps so it doesn't permanently flood standard Info logs,
	// but is easily accessible when we run with -v or debug mode.
	tm.logger.Debug("--- MANAGER SYSTEM PROMPT ---\n" + dynamicSystemPrompt)
	for i, h := range history {
		if len(h.Parts) > 0 {
			tm.logger.Debug(fmt.Sprintf("--- MANAGER HISTORY [%d] (%s) ---\n%s", i, h.Role, h.Parts[0].Text))
		}
	}
	// ------------------------------------------

	// Inject the fused dynamic system prompt for the Coordinator
	wrappedThinkSpace := WrapSpace(thinkSpace, dynamicSystemPrompt)

	return coordinator.ExecuteTurn(ctx, thread, manifest, wrappedThinkSpace, history, ui)
}
