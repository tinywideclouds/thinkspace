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
	engineType := flag.String("engine", "gogit", "State engine backend ('gogit' or 'exec')")

	useSkeleton := flag.Bool("use-skeleton", false, "DEV SHORTCUT: Seed local directories using the internal skeleton configuration")
	flag.Parse()

	rootMode := true
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	_ = godotenv.Load()

	client, err := genai.NewClient(ctx, nil)
	if err != nil {
		logger.Error("Failed to initialize GenAI client", "error", err)
		os.Exit(1)
	}
	modelClient := llm.NewGenAIClient(client)

	homeDirectory, err := os.UserHomeDir()
	if err != nil {
		logger.Error("Failed to get home directory", "error", err)
		os.Exit(1)
	}

	baseRoot := filepath.Join(homeDirectory, "Documents", *rootName)
	configurationsDirectory := filepath.Join(baseRoot, "configs")

	// Ensure base directories exist physically
	_ = os.MkdirAll(configurationsDirectory, 0755)

	config.ScaffoldDefaults(logger, configurationsDirectory)

	factory := func(repoRoot string) workspace.ChatEngine {
		if *engineType == "exec" {
			return gitfs.NewGoExecEngine(repoRoot, rootMode)
		}
		return gitfs.NewGoGitEngine(repoRoot, rootMode)
	}

	serviceManager := workspace.NewServiceManager(logger, baseRoot, factory)

	registry := config.NewRegistry()
	if err := registry.LoadDirectory(configurationsDirectory); err != nil {
		logger.Error("Failed to load configurations", "error", err)
	}

	if *useSkeleton {
		config.ApplyDevSkeleton(logger, serviceManager)
	}

	var supportedDomains []string
	for domain := range registry.GetAllConfigs() {
		supportedDomains = append(supportedDomains, domain)
	}
	appConfig := workspace.AppConfig{
		SupportedDomains: supportedDomains,
	}

	llmAdapter := llm.NewAdapter(modelClient)
	fanOutFlow := flows.NewFanOutFlow(logger)

	mux := http.NewServeMux()

	managementAPI := api.NewManagementAPI(registry, serviceManager, appConfig)
	managementAPI.RegisterHandlers(mux)

	webSocketServer := api.NewServer(logger, registry, serviceManager, llmAdapter, fanOutFlow, modelClient)
	webSocketServer.RegisterHandlers(mux)

	serverAddress := fmt.Sprintf(":%d", *port)
	logger.Info("🚀 ThinkSpace Server listening", "addr", serverAddress)

	if err := http.ListenAndServe(serverAddress, mux); err != nil {
		logger.Error("Server stopped", "error", err)
	}
}
