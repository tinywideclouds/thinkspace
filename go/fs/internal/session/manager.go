package session

import (
	"context"
	"log/slog"
	"sync"

	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// Manager acts as the top-level orchestrator, encapsulating dependencies
// and managing the lifecycle and caching of active chat sessions.
type Manager struct {
	logger           *slog.Logger
	registry         *config.Registry
	workspaceManager *workspace.ServiceManager
	modelClient      llm.ModelClient
	fanOutFlow       flows.Flow
	digestManager    *DigestManager

	sessionMu sync.Mutex
	sessions  map[string]*ChatSession
}

func NewManager(
	logger *slog.Logger,
	registry *config.Registry,
	workspaceManager *workspace.ServiceManager,
	modelClient llm.ModelClient,
	fanOutFlow flows.Flow,
	digestManager *DigestManager,
) *Manager {
	return &Manager{
		logger:           logger,
		registry:         registry,
		workspaceManager: workspaceManager,
		modelClient:      modelClient,
		fanOutFlow:       fanOutFlow,
		digestManager:    digestManager,
		sessions:         make(map[string]*ChatSession),
	}
}

// GetSession retrieves an active session from the cache or initializes a new one.
func (m *Manager) GetSession(ctx context.Context, spaceID, chatID string) (*ChatSession, error) {
	sessionID := spaceID + ":" + chatID

	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()

	if s, exists := m.sessions[sessionID]; exists {
		return s, nil
	}

	s, err := NewChatSession(
		ctx,
		m.logger,
		m.registry,
		m.workspaceManager,
		m.modelClient,
		m.fanOutFlow,
		m.digestManager,
		spaceID,
		chatID,
	)
	if err != nil {
		return nil, err
	}

	m.sessions[sessionID] = s
	return s, nil
}

// RunTurn executes a single conversational loop for the requested session.
func (m *Manager) RunTurn(ctx context.Context, spaceID, chatID, prompt string, ui UserInterface, emitter telemetry.Emitter) error {
	session, err := m.GetSession(ctx, spaceID, chatID)
	if err != nil {
		return err
	}
	return session.RunTurn(ctx, prompt, ui, emitter)
}
