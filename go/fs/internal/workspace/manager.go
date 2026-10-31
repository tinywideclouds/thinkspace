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

	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

type EngineFactory func(repoRoot string) ChatEngine

type ServiceManager struct {
	logger        *slog.Logger
	baseRoot      string
	engineFactory EngineFactory
	graphRepo     thinkgraph.GraphRepository
	emitter       telemetry.Emitter
	services      map[string]*Service
	mu            sync.Mutex
}

func NewServiceManager(
	logger *slog.Logger,
	baseRoot string,
	factory EngineFactory,
	graphRepo thinkgraph.GraphRepository,
	emitter telemetry.Emitter,
) *ServiceManager {
	return &ServiceManager{
		logger:        logger,
		baseRoot:      baseRoot,
		engineFactory: factory,
		graphRepo:     graphRepo,
		emitter:       emitter,
		services:      make(map[string]*Service),
	}
}

func (manager *ServiceManager) WorkspaceRoot() string {
	return manager.baseRoot
}

// GraphRepository returns the raw underlying graph database connection.
func (manager *ServiceManager) GraphRepository() thinkgraph.GraphRepository {
	return manager.graphRepo
}

func (manager *ServiceManager) GetService(spaceID string) *Service {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	if service, exists := manager.services[spaceID]; exists {
		return service
	}

	spaceRoot := filepath.Join(manager.baseRoot, spaceID)
	engine := manager.engineFactory(spaceRoot)

	builder := NewWorkspaceGraphBuilder(engine)
	var dispatcher *thinkgraph.Dispatcher

	if manager.graphRepo != nil {
		graphStore := thinkgraph.NewGraphStore(manager.graphRepo, builder)
		ledger := thinkgraph.NewFileLedger(1000, manager.baseRoot)
		dispatcher = thinkgraph.NewDispatcher(graphStore, ledger)
	}

	service := NewService(manager.logger, engine, manager.baseRoot, dispatcher, manager.emitter)
	manager.services[spaceID] = service
	return service
}

func (manager *ServiceManager) ListSpaces(ctx context.Context) ([]string, error) {
	entries, err := os.ReadDir(manager.baseRoot)
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

func (manager *ServiceManager) GetSpaceState(spaceID string) (SpaceState, error) {
	var state SpaceState
	path := filepath.Join(manager.baseRoot, spaceID, "space.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return state, err
	}
	err = json.Unmarshal(data, &state)
	return state, err
}

func (manager *ServiceManager) UpdateSpaceState(spaceID string, updateFn func(*SpaceState)) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	state, _ := manager.GetSpaceState(spaceID)

	updateFn(&state)

	path := filepath.Join(manager.baseRoot, spaceID, "space.json")
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling space state: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing space state: %w", err)
	}

	return nil
}

func (manager *ServiceManager) ListChats(ctx context.Context, spaceID string) ([]ChatMeta, error) {
	chatsDir := filepath.Join(manager.baseRoot, spaceID, "chats")

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
				manager.logger.WarnContext(ctx, "failed to parse meta.json", "chat_id", chatID, "error", unmarshalErr)
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

func (manager *ServiceManager) CreateChat(ctx context.Context, spaceID string, requestedName string) (ChatMeta, error) {
	chats, err := manager.ListChats(ctx, spaceID)
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

	id := uuid.NewV7()
	idStr := id.String()
	meta := ChatMeta{
		ID:           idStr,
		Name:         name,
		CreatedAt:    time.Now().UTC(),
		ActiveBranch: "main",
		Branches: map[string]BranchMeta{
			"main": {
				ParentBranch: "",
				ForkPoint:    "",
			},
		},
	}

	chatDir := filepath.Join(manager.baseRoot, spaceID, "chats", idStr)
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

func (manager *ServiceManager) GetChatMeta(spaceID, chatID string) (ChatMeta, error) {
	chatsDir := filepath.Join(manager.baseRoot, spaceID, "chats")
	metaPath := filepath.Join(chatsDir, chatID, "meta.json")

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return ChatMeta{}, err
	}

	var meta ChatMeta
	err = json.Unmarshal(data, &meta)
	return meta, err
}

func (manager *ServiceManager) UpdateChatMeta(spaceID, chatID string, updateFn func(*ChatMeta)) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	meta, err := manager.GetChatMeta(spaceID, chatID)
	if err != nil {
		return err
	}

	updateFn(&meta)

	chatsDir := filepath.Join(manager.baseRoot, spaceID, "chats")
	metaPath := filepath.Join(chatsDir, chatID, "meta.json")

	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metaPath, metaBytes, 0644)
}
