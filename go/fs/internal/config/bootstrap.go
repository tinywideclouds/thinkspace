package config

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// ApplyDevSkeleton provisions the physical directory structure and the space.json metadata directly via the ServiceManager.
func ApplyDevSkeleton(logger *slog.Logger, manager *workspace.ServiceManager) {
	skeleton, err := GetEmbeddedSkeleton()
	if err != nil {
		logger.Error("Failed to parse embedded skeleton file", "error", err)
		return
	}

	logger.Info("🌱 Seeding developer skeleton", "space", skeleton.Space, "chat", skeleton.Chat, "domain", skeleton.Domain)

	spaceDir := filepath.Join(manager.WorkspaceRoot(), skeleton.Space)
	chatDir := filepath.Join(spaceDir, "chats", skeleton.Chat)

	if err := os.MkdirAll(chatDir, 0755); err != nil {
		logger.Error("Failed to seed skeleton directories", "error", err)
		return
	}

	if err := manager.UpdateSpaceState(skeleton.Space, func(state *workspace.SpaceState) {
		state.Domain = skeleton.Domain
	}); err != nil {
		logger.Error("Failed to set skeleton state", "error", err)
	}
}
