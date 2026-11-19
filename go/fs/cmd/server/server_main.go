package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"github.com/dgraph-io/badger/v4"
	"github.com/joho/godotenv"
	"google.golang.org/genai"

	"github.com/tinywideclouds.com/thinkspace/internal/api"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/db"
	"github.com/tinywideclouds.com/thinkspace/internal/gitfs"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/llm/gemini"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

func main() {
	port := flag.Int("port", 8080, "Port for the ThinkSpace WebServer")
	rootName := flag.String("root", "thinkspace-root", "Root directory for the ThinkSpace repositories")
	engineType := flag.String("engine", "gogit", "State engine backend ('gogit' or 'exec')")

	useSkeleton := flag.Bool("use-skeleton", false, "DEV SHORTCUT: Seed local directories using the internal skeleton configuration")
	cleanDB := flag.Bool("clean", false, "DEV SHORTCUT: Wipe local BadgerDB graph index on startup for a fresh clean state")
	debugLLM := flag.Bool("debug-llm", false, "DEV SHORTCUT: Record full compiled ThinkBundle context to temp Markdown files on every LLM call")
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

	var modelClient llm.ModelClient = gemini.NewAdapter(client)
	if *debugLLM {
		logger.Info("🔍 LLM debug recording enabled: compiled ThinkBundle contexts will be saved to temp files")
		modelClient = llm.NewLoggingModelClient(logger, "", modelClient)
	}

	homeDirectory, err := os.UserHomeDir()
	if err != nil {
		logger.Error("Failed to get home directory", "error", err)
		os.Exit(1)
	}

	baseRoot := filepath.Join(homeDirectory, "Documents", *rootName)
	configurationsDirectory := filepath.Join(baseRoot, "configs")
	dbPath := filepath.Join(baseRoot, ".graphdb")

	if *cleanDB {
		logger.Info("🧹 Clean flag active: wiping BadgerDB graph index directory", "path", dbPath)
		if err := os.RemoveAll(dbPath); err != nil {
			logger.Warn("Failed removing existing database directory", "error", err)
		}
	}

	if err := os.MkdirAll(configurationsDirectory, 0755); err != nil {
		logger.Error("Failed to create configurations directory", "error", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(dbPath, 0755); err != nil {
		logger.Error("Failed to create database directory", "error", err)
		os.Exit(1)
	}

	config.ScaffoldDefaults(logger, configurationsDirectory)

	opts := badger.DefaultOptions(dbPath).WithLoggingLevel(badger.WARNING)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		logger.Error("Failed to initialize BadgerDB", "error", err)
		os.Exit(1)
	}
	defer func() {
		if closeErr := graphDB.Close(); closeErr != nil {
			logger.Error("Failed to close BadgerDB", "error", closeErr)
		}
	}()

	factory := func(repoRoot string) workspace.ChatVersioner {
		if *engineType == "exec" {
			return gitfs.NewGoExecEngine(repoRoot, rootMode)
		}
		return gitfs.NewGoGitEngine(repoRoot, rootMode)
	}

	emitter := telemetry.NewSlogEmitter(logger)
	serviceManager := workspace.NewServiceManager(logger, baseRoot, factory, graphDB, emitter)

	registry := config.NewRegistry()
	if err := registry.LoadDirectory(configurationsDirectory); err != nil {
		logger.Error("Failed to load configurations", "error", err)
	}

	if *useSkeleton {
		config.ApplyDevSkeleton(logger, serviceManager)
	}

	allConfigs := registry.GetAllConfigs()
	supportedDomains := make([]string, 0, len(allConfigs))
	for domain := range allConfigs {
		supportedDomains = append(supportedDomains, domain)
	}
	sort.Strings(supportedDomains)
	appConfig := workspace.AppConfig{
		SupportedDomains: supportedDomains,
	}

	var fanOutCfg flows.FanOutConfig
	if err := registry.DecodeFlowConfig("fanout", &fanOutCfg); err != nil {
		logger.Warn("Failed to decode fanout config, using defaults", "error", err)
	}
	fanOutFlow := flows.NewFanOutFlow(logger, fanOutCfg)

	var digestCfg session.DigestConfig
	if err := registry.DecodeCoreConfig("digest", &digestCfg); err != nil {
		logger.Warn("Failed to decode digest config, using defaults", "error", err)
	}
	digestManager := session.NewDigestManager(logger, modelClient, digestCfg)

	turnManager := session.NewManager(logger, registry, serviceManager, modelClient, fanOutFlow, digestManager)

	mux := http.NewServeMux()

	managementAPI := api.NewManagementAPI(registry, serviceManager, appConfig)
	managementAPI.RegisterHandlers(mux)

	webSocketServer := api.NewServer(logger, registry, serviceManager, turnManager)
	webSocketServer.RegisterHandlers(mux)

	serverAddress := fmt.Sprintf(":%d", *port)
	logger.Info("🚀 ThinkSpace Server listening", "addr", serverAddress)

	if err := http.ListenAndServe(serverAddress, mux); err != nil {
		logger.Error("Server stopped", "error", err)
	}
}
