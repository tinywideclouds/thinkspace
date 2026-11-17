package config

import (
	"embed"
	"errors"
	"log/slog"
	"os"
	"path"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

//go:embed defaults/*.yaml
var defaultConfigs embed.FS

// DevSkeleton defines the local developer environment seeding parameters.
type DevSkeleton struct {
	Space      string `yaml:"space"`
	Chat       string `yaml:"chat"`
	ThinkState string `yaml:"think_state"`
}

// ConfigVersion is used to quickly peek at a block's version for migration
type ConfigVersion struct {
	Version int `yaml:"version"`
}

// ScaffoldDefaults writes default configuration files to the disk if they do not already exist.
// It also checks the version of existing files and upgrades them if the embedded version is newer.
func ScaffoldDefaults(logger *slog.Logger, configurationsDirectory string) {
	if logger == nil {
		logger = slog.Default()
	}

	if err := os.MkdirAll(configurationsDirectory, 0755); err != nil {
		logger.Error("Failed to create configurations directory", "dir", configurationsDirectory, "error", err)
		return
	}

	filesToScaffold := []string{"golang.yaml", "concept.yaml", "intake.yaml", "fanout.yaml", "digest.yaml"}

	for _, filename := range filesToScaffold {
		targetPath := filepath.Join(configurationsDirectory, filename)
		embedPath := path.Join("defaults", filename)

		embeddedContent, err := defaultConfigs.ReadFile(embedPath)
		if err != nil {
			logger.Error("Failed to read embedded configuration", "file", filename, "error", err)
			continue
		}

		if _, statErr := os.Stat(targetPath); errors.Is(statErr, os.ErrNotExist) {
			logger.Info("Scaffolding default configuration file", "file", filename)
			if writeErr := os.WriteFile(targetPath, embeddedContent, 0644); writeErr != nil {
				logger.Error("Failed to write default configuration", "file", filename, "error", writeErr)
			}
			continue
		} else if statErr != nil {
			logger.Error("Failed to stat existing configuration file", "file", filename, "error", statErr)
			continue
		}

		// VERSION CHECKING: Peek inside the root keys to check versions
		diskContent, err := os.ReadFile(targetPath)
		if err != nil {
			logger.Error("Failed to read existing configuration file", "file", filename, "error", err)
			continue
		}

		var diskMeta map[string]ConfigVersion
		var embedMeta map[string]ConfigVersion

		if err := yaml.Unmarshal(diskContent, &diskMeta); err != nil {
			logger.Warn("Existing configuration file has invalid YAML; skipping auto-upgrade", "file", filename, "error", err)
			continue
		}
		if err := yaml.Unmarshal(embeddedContent, &embedMeta); err != nil {
			logger.Error("Failed to parse embedded configuration metadata", "file", filename, "error", err)
			continue
		}

		embedVersion := 0
		for _, v := range embedMeta {
			if v.Version > embedVersion {
				embedVersion = v.Version
			}
		}

		diskVersion := 0
		for _, v := range diskMeta {
			if v.Version > diskVersion {
				diskVersion = v.Version
			}
		}

		// If embedded version is strictly greater, backup and overwrite
		if embedVersion > diskVersion {
			logger.Info("Upgrading configuration file", "file", filename, "old_version", diskVersion, "new_version", embedVersion)

			backupPath := targetPath + ".bak"
			if backupErr := os.WriteFile(backupPath, diskContent, 0644); backupErr != nil {
				logger.Error("Failed to write configuration backup; aborting upgrade", "file", filename, "error", backupErr)
				continue
			}

			if writeErr := os.WriteFile(targetPath, embeddedContent, 0644); writeErr != nil {
				logger.Error("Failed to upgrade configuration", "file", filename, "error", writeErr)
			}
		}
	}
}

// GetEmbeddedSkeleton returns the embedded skeleton configuration for dev bootstrapping.
func GetEmbeddedSkeleton() (DevSkeleton, error) {
	var skeleton DevSkeleton
	content, err := defaultConfigs.ReadFile("defaults/skeleton.yaml")
	if err != nil {
		return skeleton, err
	}
	err = yaml.Unmarshal(content, &skeleton)
	return skeleton, err
}
