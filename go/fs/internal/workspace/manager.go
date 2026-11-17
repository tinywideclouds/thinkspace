package workspace

import (
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
	"time"
	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

type EngineFactory func(repoRoot string) ChatVersioner

type ServiceManager struct {
	logger        *slog.Logger
	baseRoot      string
	engineFactory EngineFactory
	graphRepo     thinkgraph.GraphRepository
	emitter       telemetry.Emitter
	services      map[string]*Service
	mu            sync.RWMutex
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

func (manager *ServiceManager) GraphRepository() thinkgraph.GraphRepository {
	return manager.graphRepo
}

func (manager *ServiceManager) GetService(spaceID string) *Service {
	manager.mu.RLock()
	if service, exists := manager.services[spaceID]; exists {
		manager.mu.RUnlock()
		return service
	}
	manager.mu.RUnlock()

	manager.mu.Lock()
	defer manager.mu.Unlock()

	if service, exists := manager.services[spaceID]; exists {
		return service
	}

	spaceRoot := filepath.Join(manager.baseRoot, spaceID)
	workspaceDir := filepath.Join(spaceRoot, "workspace")
	if err := os.MkdirAll(workspaceDir, 0755); err != nil && manager.logger != nil {
		manager.logger.Error("failed to create workspace directory", "space_id", spaceID, "error", err)
	}

	var engine ChatVersioner
	if manager.engineFactory != nil {
		engine = manager.engineFactory(workspaceDir)
	}

	var dispatcher *thinkgraph.Dispatcher
	if manager.graphRepo != nil {
		graphStore := thinkgraph.NewGraphStore(manager.graphRepo, nil)
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
		if errors.Is(err, os.ErrNotExist) {
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

func (manager *ServiceManager) getSpaceStateLocked(spaceID string) (SpaceState, error) {
	var state SpaceState
	path := filepath.Join(manager.baseRoot, spaceID, "space.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, fmt.Errorf("unmarshaling space state: %w", err)
	}
	return state, nil
}

func (manager *ServiceManager) GetSpaceState(spaceID string) (SpaceState, error) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return manager.getSpaceStateLocked(spaceID)
}

func (manager *ServiceManager) UpdateSpaceState(spaceID string, updateFn func(*SpaceState)) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	state, err := manager.getSpaceStateLocked(spaceID)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reading existing space state: %w", err)
	}

	updateFn(&state)

	path := filepath.Join(manager.baseRoot, spaceID, "space.json")
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling space state: %w", err)
	}

	if err := writeFileAtomic(path, data); err != nil {
		return fmt.Errorf("writing space state: %w", err)
	}

	return nil
}

func (manager *ServiceManager) getSpaceSettingsLocked(spaceID string) (SpaceSettings, error) {
	var settings SpaceSettings
	path := filepath.Join(manager.baseRoot, spaceID, "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SpaceSettings{
				ReviewPolicy: ReviewPolicySettings{
					Active:         true,
					MinTokens:      200,
					IncludePersona: false,
				},
				Bundle: BundleSettings{
					MaxWorkingMemoryEvents: 50,
					IncludeMapbookOnBoot:   true,
				},
			}, nil
		}
		return settings, err
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return settings, fmt.Errorf("unmarshaling space settings: %w", err)
	}
	return settings, nil
}

func (manager *ServiceManager) GetSpaceSettings(spaceID string) (SpaceSettings, error) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return manager.getSpaceSettingsLocked(spaceID)
}

func (manager *ServiceManager) UpdateSpaceSettings(spaceID string, updateFn func(*SpaceSettings)) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	settings, err := manager.getSpaceSettingsLocked(spaceID)
	if err != nil {
		return fmt.Errorf("reading existing space settings: %w", err)
	}

	updateFn(&settings)

	path := filepath.Join(manager.baseRoot, spaceID, "settings.json")
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling space settings: %w", err)
	}

	if err := writeFileAtomic(path, data); err != nil {
		return fmt.Errorf("writing space settings: %w", err)
	}

	return nil
}

func (manager *ServiceManager) listChatsLocked(ctx context.Context, spaceID string) ([]ChatMeta, error) {
	chatsDir := filepath.Join(manager.baseRoot, spaceID, "chats")

	entries, err := os.ReadDir(chatsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
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

		if data, readErr := os.ReadFile(metaPath); readErr == nil {
			if unmarshalErr := json.Unmarshal(data, &meta); unmarshalErr != nil && manager.logger != nil {
				manager.logger.WarnContext(ctx, "failed to parse meta.json", "chat_id", chatID, "error", unmarshalErr)
			}
		} else {
			if !errors.Is(readErr, os.ErrNotExist) && manager.logger != nil {
				manager.logger.WarnContext(ctx, "failed to read meta.json", "chat_id", chatID, "error", readErr)
			}
			if info, infoErr := entry.Info(); infoErr == nil {
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

func (manager *ServiceManager) ListChats(ctx context.Context, spaceID string) ([]ChatMeta, error) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return manager.listChatsLocked(ctx, spaceID)
}

func (manager *ServiceManager) CreateChat(ctx context.Context, spaceID string, requestedName string) (ChatMeta, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	chats, err := manager.listChatsLocked(ctx, spaceID)
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
	}

	metaPath := filepath.Join(manager.baseRoot, spaceID, "chats", idStr, "meta.json")
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return ChatMeta{}, fmt.Errorf("marshaling meta.json: %w", err)
	}

	if err := writeFileAtomic(metaPath, metaBytes); err != nil {
		return ChatMeta{}, fmt.Errorf("writing meta.json: %w", err)
	}

	return meta, nil
}

func (manager *ServiceManager) getChatMetaLocked(spaceID, chatID string) (ChatMeta, error) {
	chatsDir := filepath.Join(manager.baseRoot, spaceID, "chats")
	metaPath := filepath.Join(chatsDir, chatID, "meta.json")

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return ChatMeta{}, err
	}

	var meta ChatMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return ChatMeta{}, fmt.Errorf("unmarshaling meta.json: %w", err)
	}
	return meta, nil
}

func (manager *ServiceManager) GetChatMeta(spaceID, chatID string) (ChatMeta, error) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return manager.getChatMetaLocked(spaceID, chatID)
}

func (manager *ServiceManager) UpdateChatMeta(spaceID, chatID string, updateFn func(*ChatMeta)) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	meta, err := manager.getChatMetaLocked(spaceID, chatID)
	if err != nil {
		return err
	}

	updateFn(&meta)

	metaPath := filepath.Join(manager.baseRoot, spaceID, "chats", chatID, "meta.json")
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling meta.json: %w", err)
	}

	return writeFileAtomic(metaPath, metaBytes)
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file in %s: %w", dir, err)
	}
	tmpName := tmpFile.Name()

	cleanup := func() {
		_ = os.Remove(tmpName)
	}

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		cleanup()
		return fmt.Errorf("writing temp file %s: %w", tmpName, err)
	}

	if err := tmpFile.Close(); err != nil {
		cleanup()
		return fmt.Errorf("closing temp file %s: %w", tmpName, err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("renaming temp file to %s: %w", path, err)
	}

	return nil
}
