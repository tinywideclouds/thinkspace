package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

type EngineFactory func(repoRoot string) ChatEngine

type ServiceManager struct {
	logger        *slog.Logger
	baseRoot      string
	engineFactory EngineFactory
	graphStore    thinkgraph.GraphRepository
	services      map[string]*Service
	mu            sync.Mutex
}

func NewServiceManager(logger *slog.Logger, baseRoot string, factory EngineFactory, graphStore thinkgraph.GraphRepository) *ServiceManager {
	return &ServiceManager{
		logger:        logger,
		baseRoot:      baseRoot,
		engineFactory: factory,
		graphStore:    graphStore,
		services:      make(map[string]*Service),
	}
}

func (m *ServiceManager) WorkspaceRoot() string {
	return m.baseRoot
}

func (m *ServiceManager) GraphStore() thinkgraph.GraphRepository {
	return m.graphStore
}

func (m *ServiceManager) GetService(spaceID string) *Service {
	m.mu.Lock()
	defer m.mu.Unlock()

	if service, exists := m.services[spaceID]; exists {
		return service
	}

	spaceRoot := filepath.Join(m.baseRoot, spaceID)
	engine := m.engineFactory(spaceRoot)

	bus := thinkgraph.NewEventBus()
	bus.Subscribe(thinkgraph.NewLedgerSubscriber(1000, m.baseRoot))
	bus.Subscribe(thinkgraph.NewDigestSubscriber(m.baseRoot))

	if m.graphStore != nil {
		bus.Subscribe(thinkgraph.NewGraphSubscriber(m.graphStore))
	}

	service := NewService(m.logger, engine, m.baseRoot, bus, m.graphStore)

	m.services[spaceID] = service
	return service
}

func (m *ServiceManager) ListSpaces(ctx context.Context) ([]string, error) {
	entries, err := os.ReadDir(m.baseRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("reading base directory: %w", err)
	}

	var spaces []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			spaces = append(spaces, entry.Name())
		}
	}
	return spaces, nil
}

func (m *ServiceManager) GetSpaceState(spaceID string) (SpaceState, error) {
	var state SpaceState
	path := filepath.Join(m.baseRoot, spaceID, "space.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return state, err
	}
	err = json.Unmarshal(data, &state)
	return state, err
}

func (m *ServiceManager) UpdateSpaceState(spaceID string, updateFn func(*SpaceState)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, _ := m.GetSpaceState(spaceID)

	updateFn(&state)

	path := filepath.Join(m.baseRoot, spaceID, "space.json")
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling space state: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing space state: %w", err)
	}

	return nil
}

func (m *ServiceManager) ListChats(ctx context.Context, spaceID string) ([]ChatMeta, error) {
	chatsDir := filepath.Join(m.baseRoot, spaceID, "chats")

	entries, err := os.ReadDir(chatsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []ChatMeta{}, nil
		}
		return nil, fmt.Errorf("reading chats directory: %w", err)
	}

	var chats []ChatMeta
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		chatID := entry.Name()
		metaPath := filepath.Join(chatsDir, chatID, "meta.json")

		meta := ChatMeta{
			ID:        chatID,
			Name:      chatID,
			CreatedAt: time.Now(),
		}

		if data, err := os.ReadFile(metaPath); err == nil {
			if unmarshalErr := json.Unmarshal(data, &meta); unmarshalErr != nil {
				m.logger.WarnContext(ctx, "failed to parse meta.json", "chat_id", chatID, "error", unmarshalErr)
			}
		} else {
			if info, err := entry.Info(); err == nil {
				meta.CreatedAt = info.ModTime()
			}
		}
		chats = append(chats, meta)
	}

	sort.Slice(chats, func(i, j int) bool {
		return chats[i].CreatedAt.After(chats[j].CreatedAt)
	})

	return chats, nil
}

func (m *ServiceManager) CreateChat(ctx context.Context, spaceID string, requestedName string) (ChatMeta, error) {
	chats, err := m.ListChats(ctx, spaceID)
	if err != nil {
		return ChatMeta{}, err
	}

	name := strings.TrimSpace(requestedName)
	if name == "" {
		baseName := "New Conversation"
		name = baseName
		counter := 1

		nameExists := func(n string) bool {
			for _, chat := range chats {
				if chat.Name == n {
					return true
				}
			}
			return false
		}

		for nameExists(name) {
			name = fmt.Sprintf("%s (%d)", baseName, counter)
			counter++
		}
	}

	id := uuid.NewV7().String()
	meta := ChatMeta{
		ID:        id,
		Name:      name,
		CreatedAt: time.Now().UTC(),
	}

	chatDir := filepath.Join(m.baseRoot, spaceID, "chats", id)
	if err := os.MkdirAll(chatDir, 0755); err != nil {
		return ChatMeta{}, fmt.Errorf("creating chat directory: %w", err)
	}

	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return ChatMeta{}, fmt.Errorf("marshaling meta.json: %w", err)
	}

	if err := os.WriteFile(filepath.Join(chatDir, "meta.json"), metaBytes, 0644); err != nil {
		return ChatMeta{}, fmt.Errorf("writing meta.json: %w", err)
	}

	return meta, nil
}

func (m *ServiceManager) GetChatMeta(spaceID, chatID string) (ChatMeta, error) {
	chatsDir := filepath.Join(m.baseRoot, spaceID, "chats")
	metaPath := filepath.Join(chatsDir, chatID, "meta.json")

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return ChatMeta{}, err
	}

	var meta ChatMeta
	err = json.Unmarshal(data, &meta)
	return meta, err
}

func (m *ServiceManager) UpdateChatMeta(spaceID, chatID string, updateFn func(*ChatMeta)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	meta, err := m.GetChatMeta(spaceID, chatID)
	if err != nil {
		return err
	}

	updateFn(&meta)

	chatsDir := filepath.Join(m.baseRoot, spaceID, "chats")
	metaPath := filepath.Join(chatsDir, chatID, "meta.json")

	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metaPath, metaBytes, 0644)
}
