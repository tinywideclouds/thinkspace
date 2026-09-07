package config

import (
	"embed"
	"log/slog"
	"os"
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

// ScaffoldDefaults writes default configuration files to the disk if they do not already exist.
func ScaffoldDefaults(logger *slog.Logger, configurationsDirectory string) {
	filesToScaffold := []string{"golang.yaml", "fanout.yaml"}

	for _, filename := range filesToScaffold {
		targetPath := filepath.Join(configurationsDirectory, filename)
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			logger.Info("Scaffolding default configuration file", "file", filename)

			content, err := defaultConfigs.ReadFile(filepath.Join("defaults", filename))
			if err != nil {
				logger.Error("Failed to read embedded configuration", "file", filename, "error", err)
				continue
			}

			if err := os.WriteFile(targetPath, content, 0644); err != nil {
				logger.Error("Failed to write default configuration", "file", filename, "error", err)
			}
		}
	}
}

// GetEmbeddedSkeleton returns the embedded skeleton configuration for dev bootstrapping.
// It is strictly an internal memory asset and never written to the user's disk.
func GetEmbeddedSkeleton() (DevSkeleton, error) {
	var skeleton DevSkeleton
	content, err := defaultConfigs.ReadFile("defaults/skeleton.yaml")
	if err != nil {
		return skeleton, err
	}
	err = yaml.Unmarshal(content, &skeleton)
	return skeleton, err
}
