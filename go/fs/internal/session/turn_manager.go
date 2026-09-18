package session

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"uuid"
)

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

func (tm *TurnManager) ExecuteTurn(ctx context.Context, spaceID, chatID, promptText string, ui UserInterface, emitter telemetry.Emitter) error {
	emitter.Emit("manager", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "log",
		Timestamp: time.Now(),
		Message:   "Initializing workspace turn...",
	})

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

	if promptText != "" {
		if err := workspaceService.LogUserPrompt(ctx, thread, promptText); err != nil {
			return fmt.Errorf("logging user prompt: %w", err)
		}
	}

	emitter.Emit("manager", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "log",
		Timestamp: time.Now(),
		Message:   "Assembling JIT context from workspace maps and ledger...",
	})

	graph, manifest, err := tm.playbackEngine.LoadState(ctx, thread)
	if err != nil {
		return fmt.Errorf("loading updated state: %w", err)
	}

	assemblyReq := chat.AssemblyRequest{
		Manifest:     manifest,
		ActiveLenses: []string{},
		RecentEvents: graph.RecentEvents,
	}

	dynamicSystemPrompt, history, err := tm.contextAssembler.Build(ctx, thinkSpace, workspaceService.WorkspaceRoot(), assemblyReq)
	if err != nil {
		return fmt.Errorf("assembling context: %w", err)
	}

	var fullContext strings.Builder
	fullContext.WriteString("=== SYSTEM PROMPT ===\n")
	fullContext.WriteString(dynamicSystemPrompt)
	fullContext.WriteString("\n\n=== CHAT HISTORY ===\n")
	for _, h := range history {
		if len(h.Parts) > 0 {
			fullContext.WriteString(fmt.Sprintf("[%s]: %s\n\n", h.Role, h.Parts[0].Text))
		}
	}

	previewID, err := workspaceService.SaveContextPreview(ctx, thread, fullContext.String())
	if err != nil {
		tm.logger.WarnContext(ctx, "failed to save context preview", "error", err)
	}

	var stickyDigests []string
	for _, d := range manifest.Digests {
		if d.IsSticky {
			stickyDigests = append(stickyDigests, d.Summary)
		}
	}

	var recentHistory []string
	for _, ev := range graph.RecentEvents {
		recentHistory = append(recentHistory, fmt.Sprintf("[%s] %s", ev.Type, ev.Content))
	}

	turnCtx := TurnContext{
		SpaceID:             spaceID,
		ChatID:              chatID,
		PreviewID:           previewID,
		ManagerSystemPrompt: thinkSpace.Config().ManagerSystemPrompt(),
		StickyDigests:       stickyDigests,
		RecentHistory:       recentHistory,
		FileContexts:        nil,
	}

	emitter.Emit("manager", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "log",
		Timestamp: time.Now(),
		Message:   "Context assembled successfully. Yielding for frontend approval...",
	})

	if !ui.ReviewContext(turnCtx) {
		tm.logger.Info("Turn cancelled by user during context review")
		return nil
	}

	workerModel := thinkSpace.Config().Roles.Worker.Model

	executor := llm.NewSubAgentExecutor(
		tm.logger,
		tm.modelClient,
		workerModel,
		thinkSpace.Config().WorkerSystemPrompt(),
		thinkSpace.Config().Roles.Worker.MaxTokens,
		thinkSpace.Patcher(),
	)

	delegator := NewDelegator(
		tm.logger,
		workspaceService,
		tm.llmAdapter,
		executor,
		tm.fanOutFlow,
		emitter,
		spaceID,
		spaceConfig.WorkerSystemPrompt(),
		flowConfig,
	)

	coordinator := NewCoordinator(
		tm.logger,
		workspaceService,
		tm.llmAdapter,
		emitter,
		delegator,
	)

	tm.logger.Info("=== MANAGER CONTEXT ASSEMBLY COMPLETE ===",
		"system_prompt_length", len(dynamicSystemPrompt),
		"history_length", len(history),
	)

	wrappedThinkSpace := WrapSpace(thinkSpace, dynamicSystemPrompt)

	return coordinator.ExecuteTurn(ctx, thread, manifest, wrappedThinkSpace, history, ui)
}
