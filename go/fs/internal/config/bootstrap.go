package config

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// ApplyDevSkeleton provisions the physical directory structure and metadata directly via the ServiceManager.
func ApplyDevSkeleton(logger *slog.Logger, manager *workspace.ServiceManager) {
	if logger == nil {
		logger = slog.Default()
	}

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
	if _, statErr := os.Stat(metaPath); errors.Is(statErr, os.ErrNotExist) {
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
	} else if statErr != nil {
		logger.Error("Failed checking skeleton meta.json", "error", statErr)
	}

	settingsPath := filepath.Join(spaceDir, "settings.json")
	if _, statErr := os.Stat(settingsPath); errors.Is(statErr, os.ErrNotExist) {
		settings := workspace.SpaceSettings{
			ReviewPolicy: workspace.ReviewPolicySettings{
				Active:    true,
				MinTokens: 200,
			},
			WorkerReviewPolicy: workspace.ReviewPolicySettings{
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
	} else if statErr != nil {
		logger.Error("Failed checking skeleton settings.json", "error", statErr)
	}

	if err := manager.UpdateSpaceState(skeleton.Space, func(state *workspace.SpaceState) {
		state.ThinkState = skeleton.ThinkState
	}); err != nil {
		logger.Error("Failed to set skeleton state", "error", err)
	}
}
