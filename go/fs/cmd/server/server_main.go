package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"google.golang.org/genai"

	"github.com/tinywideclouds.com/thinkspace/internal/api"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/gitfs"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace/flows"
)

func main() {
	port := flag.Int("port", 8080, "Port for the ThinkSpace WebServer")
	rootName := flag.String("root", "thinkspace-root", "Root directory for the ThinkSpace repositories")
	spaceName := flag.String("space", "sandbox", "The think space to use")
	engineType := flag.String("engine", "gogit", "State engine backend ('gogit' or 'exec')")
	flag.Parse()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	_ = godotenv.Load()

	client, err := genai.NewClient(ctx, nil)
	if err != nil {
		logger.Error("Failed to initialize GenAI client", "error", err)
		os.Exit(1)
	}
	modelClient := llm.NewGenAIClient(client)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Error("Failed to get home directory", "error", err)
		os.Exit(1)
	}

	repoRoot := filepath.Join(homeDir, "Documents", *rootName, *spaceName)
	configsDir := filepath.Join(homeDir, "Documents", "thinkspace", "configs")
	_ = os.MkdirAll(repoRoot, 0755)

	var stateEngine workspace.StateEngine
	switch *engineType {
	case "exec":
		stateEngine = gitfs.NewGoExecEngine()
	default:
		stateEngine = gitfs.NewGoGitEngine()
	}

	// 1. Initialize Registry and load domains
	registry := config.NewRegistry()
	if err := registry.LoadDirectory(configsDir); err != nil {
		logger.Error("Failed to load configs", "error", err)
	}

	// 2. Wire up shared dependencies
	llmMgr := llm.NewManager(modelClient)
	fanOutFlow := flows.NewFanOutFlow("FanOut", logger)
	workspaceService := workspace.NewService(logger, stateEngine, repoRoot)

	// 3. Initialize the new API Server
	srv := api.NewServer(logger, registry, workspaceService, llmMgr, fanOutFlow, modelClient)

	serverAddr := fmt.Sprintf(":%d", *port)
	logger.Info("🚀 ThinkSpace Server listening", "addr", serverAddr)

	// 4. Start serving the HTTP/WebSocket routes
	if err := http.ListenAndServe(serverAddr, srv.Handler()); err != nil {
		logger.Error("Server stopped", "error", err)
	}
}
