package session

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type SystemEngine struct {
	logger           *slog.Logger
	session          *ChatSession
	registry         *config.Registry
	workspaceService *workspace.Service
}

func NewSystemEngine(
	logger *slog.Logger,
	session *ChatSession,
	registry *config.Registry,
	workspaceService *workspace.Service,
) *SystemEngine {
	return &SystemEngine{
		logger:           logger,
		session:          session,
		registry:         registry,
		workspaceService: workspaceService,
	}
}

func (e *SystemEngine) Execute(ctx context.Context, address *thinkgraph.ChatAddress, turnID uuid.UUID, catalystID uuid.UUID, signal signals.EngineSignal, ui UserInterface, emitter telemetry.Emitter) (signals.EngineSignal, error) {
	sig, ok := signal.(*signals.SignalTransition)
	if !ok {
		return nil, fmt.Errorf("SystemEngine expected SignalTransition, got %T", signal)
	}

	targetThinkState := sig.TargetThinkState
	targetSpace, exists := e.registry.GetThinkState(targetThinkState)
	if !exists {
		ui.OnTextChunk(workspace.NodeChunk{NodeID: turnID, Text: fmt.Sprintf("\n\n⚠️ **System Error:** Think state '%s' does not exist in registry. Transition aborted.\n", targetThinkState)})
		return nil, fmt.Errorf("invalid transition target: %s", targetThinkState)
	}

	chatDir := filepath.Join(e.workspaceService.WorkspaceRoot(), e.session.SpaceID, "chats", address.ChatID)

	e.session.mu.Lock()
	hasInitialized := false
	for _, d := range e.session.Meta.InitializedThinkStates {
		if d == targetThinkState {
			hasInitialized = true
			break
		}
	}

	if !hasInitialized {
		if initErr := targetSpace.Initialize(ctx, chatDir); initErr != nil {
			e.session.mu.Unlock()
			return nil, fmt.Errorf("failed to initialize domain %s: %w", targetThinkState, initErr)
		}
		e.session.Meta.InitializedThinkStates = append(e.session.Meta.InitializedThinkStates, targetThinkState)
	}

	e.session.Meta.ActiveThinkState = targetThinkState
	if err := e.session.flushMetaLocked(); err != nil {
		e.logger.WarnContext(ctx, "failed to flush meta after domain transition", "error", err)
	}
	e.session.mu.Unlock()

	if emitter != nil {
		emitter.Emit("manager", telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "thinkstate_transition",
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("Workspace transitioned to %s domain.", targetThinkState),
			Payload: flows.TransitionTelemetry{
				TargetThinkState: targetThinkState,
			},
		})
	}

	sysAddress := e.session.GetSystemAddress()

	sysEv := thinkgraph.NewEvent(thinkgraph.EventSystem, fmt.Sprintf("Workspace transitioned to %s", targetThinkState))
	if err := e.workspaceService.Dispatcher().AppendEvent(ctx, sysAddress, sysEv); err != nil {
		e.logger.WarnContext(ctx, "failed appending transition system event", "error", err)
	}

	if graph := e.workspaceService.Graph(); graph != nil && catalystID != uuid.Nil() {
		mut := thinkgraph.MetadataMutation{
			Metadata: map[string]string{"transitioned_to": targetThinkState},
		}
		mut.TargetID = catalystID
		mut.Type = thinkgraph.MutationMetadata
		mut.Timestamp = time.Now().UTC()

		if err := e.workspaceService.SaveMutation(ctx, address, mut); err != nil {
			e.logger.ErrorContext(ctx, "failed saving transition metadata mutation", "error", err, "catalyst_id", catalystID)
		}

		nodes, err := graph.GetNodes(ctx, address, []string{catalystID.String()})
		if err == nil && len(nodes) > 0 {
			if ui != nil {
				ui.StreamGraphEvent(address, nodes[0])
			}
		}
	}

	return &signals.SignalExecuteChat{}, nil
}
