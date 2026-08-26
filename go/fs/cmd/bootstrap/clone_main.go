package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/tinywideclouds.com/thinkspace/internal/bootstrap"
)

func main() {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	homeDir, _ := os.UserHomeDir()
	workspaceRoot := filepath.Join(homeDir, "Documents", "thinkspace", "sandbox-fixture")

	// The public template repo containing your code and pre-baked chats/ folder
	fixtureRepoURL := "https://github.com/tinywideclouds/thinkspace-fixture-api.git"

	// Clean existing test state
	os.RemoveAll(workspaceRoot)

	// Use the production Git bootstrapper
	gitBootstrapper := bootstrap.NewGitBootstrap(logger)
	if err := gitBootstrapper.FromRemote(ctx, fixtureRepoURL, workspaceRoot); err != nil {
		fmt.Printf("❌ Bootstrap failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Deterministic state loaded. Ready to test Manifests and Lenses!")
}
