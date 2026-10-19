package config

import (
	"embed"
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
	Space  string `yaml:"space"`
	Chat   string `yaml:"chat"`
	Domain string `yaml:"domain"`
}

// ConfigVersion is used to quickly peek at a block's version for migration
type ConfigVersion struct {
	Version int `yaml:"version"`
}

// ScaffoldDefaults writes default configuration files to the disk if they do not already exist.
// It also checks the version of existing files and upgrades them if the embedded version is newer.
func ScaffoldDefaults(logger *slog.Logger, configurationsDirectory string) {
	filesToScaffold := []string{"golang.yaml", "concept.yaml", "fanout.yaml", "digest.yaml"}

	for _, filename := range filesToScaffold {
		targetPath := filepath.Join(configurationsDirectory, filename)
		embedPath := path.Join("defaults", filename)

		embeddedContent, err := defaultConfigs.ReadFile(embedPath)
		if err != nil {
			logger.Error("Failed to read embedded configuration", "file", filename, "error", err)
			continue
		}

		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			logger.Info("Scaffolding default configuration file", "file", filename)
			if err := os.WriteFile(targetPath, embeddedContent, 0644); err != nil {
				logger.Error("Failed to write default configuration", "file", filename, "error", err)
			}
			continue
		}

		// VERSION CHECKING: Peek inside the root keys to check versions
		diskContent, err := os.ReadFile(targetPath)
		if err == nil {
			var diskMeta map[string]ConfigVersion
			var embedMeta map[string]ConfigVersion

			_ = yaml.Unmarshal(diskContent, &diskMeta)
			_ = yaml.Unmarshal(embeddedContent, &embedMeta)

			// Assume one primary block per file for versioning
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
				_ = os.WriteFile(backupPath, diskContent, 0644)

				if err := os.WriteFile(targetPath, embeddedContent, 0644); err != nil {
					logger.Error("Failed to upgrade configuration", "file", filename, "error", err)
				}
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
