package config

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// ApplyDevSkeleton provisions the physical directory structure and metadata directly via the ServiceManager.
func ApplyDevSkeleton(logger *slog.Logger, manager *workspace.ServiceManager) {
	skeleton, err := GetEmbeddedSkeleton()
	if err != nil {
		logger.Error("Failed to parse embedded skeleton file", "error", err)
		return
	}

	logger.Info("🌱 Seeding developer skeleton", "space", skeleton.Space, "chat", skeleton.Chat, "think_state", skeleton.ThinkState)

	spaceDir := filepath.Join(manager.WorkspaceRoot(), skeleton.Space)
	chatDir := filepath.Join(spaceDir, "chats", skeleton.Chat)

	if err := os.MkdirAll(chatDir, 0755); err != nil {
		logger.Error("Failed to seed skeleton directories", "error", err)
		return
	}

	metaPath := filepath.Join(chatDir, "meta.json")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		meta := workspace.ChatMeta{
			ID:               skeleton.Chat,
			Name:             skeleton.Chat,
			CreatedAt:        time.Now().UTC(),
			ActiveThinkState: skeleton.ThinkState,
			ActiveBranch:     "main",
		}

		if metaBytes, err := json.MarshalIndent(meta, "", "  "); err == nil {
			if writeErr := os.WriteFile(metaPath, metaBytes, 0644); writeErr != nil {
				logger.Error("Failed to write skeleton meta.json", "error", writeErr)
			}
		} else {
			logger.Error("Failed to marshal skeleton meta.json", "error", err)
		}
	}

	settingsPath := filepath.Join(spaceDir, "settings.json")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		settings := workspace.SpaceSettings{
			ReviewPolicy: workspace.ReviewPolicySettings{
				Active:    true,
				MinTokens: 200,
			},
		}

		if settingsBytes, err := json.MarshalIndent(settings, "", "  "); err == nil {
			if writeErr := os.WriteFile(settingsPath, settingsBytes, 0644); writeErr != nil {
				logger.Error("Failed to write skeleton settings.json", "error", writeErr)
			}
		} else {
			logger.Error("Failed to marshal skeleton settings.json", "error", err)
		}
	}

	if err := manager.UpdateSpaceState(skeleton.Space, func(state *workspace.SpaceState) {
		state.ThinkState = skeleton.ThinkState
	}); err != nil {
		logger.Error("Failed to set skeleton state", "error", err)
	}
}
