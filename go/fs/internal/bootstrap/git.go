package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
)

// GitBootstrap handles the initialization of new ThinkSpace environments from remote Git repositories.
type GitBootstrap struct {
	logger *slog.Logger
}

func NewGitBootstrap(logger *slog.Logger) *GitBootstrap {
	return &GitBootstrap{
		logger: logger,
	}
}

// FromRemote clones an existing Git repository and prepares it for ThinkSpace.
func (b *GitBootstrap) FromRemote(ctx context.Context, repoURL, targetDir string) error {
	b.logger.InfoContext(ctx, "bootstrapping workspace from remote", slog.String("url", repoURL), slog.String("target", targetDir))

	// 1. Ensure target directory doesn't already exist to avoid clone conflicts
	if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
		return fmt.Errorf("target directory already exists: %s", targetDir)
	}

	// 2. Perform the Git Clone
	cmd := exec.CommandContext(ctx, "git", "clone", repoURL, targetDir)

	// Capture output for debugging if it fails
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %w\n%s", err, string(output))
	}

	// 3. Scaffold the ThinkSpace specific directories
	chatsDir := filepath.Join(targetDir, "chats")
	if err := os.MkdirAll(chatsDir, 0755); err != nil {
		return fmt.Errorf("failed to scaffold chats directory: %w", err)
	}

	b.logger.InfoContext(ctx, "workspace successfully bootstrapped", slog.String("target", targetDir))
	return nil
}
