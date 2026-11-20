package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/dgraph-io/badger/v4"
	"github.com/joho/godotenv"
	"google.golang.org/genai"

	"github.com/tinywideclouds.com/thinkspace/internal/cli"
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
	chatName := flag.String("chat", "unit-circle-test", "The name of the initial exploration chat")
	engineType := flag.String("engine", "gogit", "State engine backend to use ('gogit' or 'exec')")
	spaceName := flag.String("space", "sandbox", "The think space to use")
	thinkStateName := flag.String("thinkstate", "intake", "The ThinkState configuration to load")
	debugLLM := flag.Bool("debug-llm", false, "DEV SHORTCUT: Record full compiled ThinkBundle context to temp Markdown files on every LLM call")
	flag.Parse()

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
		logger.Error("Failed to get user home directory", "error", err)
		os.Exit(1)
	}

	baseRoot := filepath.Join(homeDirectory, "Documents", "thinkspace")
	repositoryRoot := filepath.Join(baseRoot, *spaceName)
	configurationsDirectory := filepath.Join(baseRoot, "configs")
	dbPath := filepath.Join(baseRoot, ".graphdb")

	if err := os.MkdirAll(repositoryRoot, 0755); err != nil {
		logger.Error("Failed to create thinkspace directory", "error", err)
		os.Exit(1)
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

	engineFactory := func(repoRoot string) workspace.ChatVersioner {
		if *engineType == "exec" {
			return gitfs.NewGoExecEngine(repoRoot, true)
		}
		return gitfs.NewGoGitEngine(repoRoot, true)
	}

	emitter := telemetry.NewSlogEmitter(logger)
	serviceManager := workspace.NewServiceManager(logger, baseRoot, engineFactory, graphDB, emitter)

	registry := config.NewRegistry()
	if err := registry.LoadDirectory(configurationsDirectory); err != nil {
		logger.Error("Failed to load configurations", "error", err)
		os.Exit(1)
	}

	if err := serviceManager.UpdateSpaceState(*spaceName, func(s *workspace.SpaceState) {
		s.ThinkState = *thinkStateName
	}); err != nil {
		logger.Error("Failed to update space state", "error", err)
		os.Exit(1)
	}

	fmt.Printf("📂 Thinkspace root: %s\n", repositoryRoot)
	if *engineType == "exec" {
		fmt.Println("⚙️  Engine: Git CLI (ExecEngine with Worktrees)")
	} else {
		fmt.Println("⚙️  Engine: go-git (GoGitEngine with Local Clones)")
	}

	isNewChat := false
	meta, err := serviceManager.GetChatMeta(*spaceName, *chatName)
	if err != nil {
		isNewChat = true
		meta, err = serviceManager.CreateChat(ctx, *spaceName, *chatName)
		if err != nil {
			logger.Error("Failed to create initial chat", "error", err)
			os.Exit(1)
		}

		if err := serviceManager.UpdateChatMeta(*spaceName, meta.ID, func(m *workspace.ChatMeta) {
			m.ActiveThinkState = *thinkStateName
		}); err != nil {
			logger.Error("Failed to persist initial chat think state", "error", err)
			os.Exit(1)
		}
	}

	var promptText string
	if isNewChat {
		fmt.Printf("🌱 Creating new exploration chat: %s (ThinkState: %s)\n", meta.ID, *thinkStateName)
		promptText = "Hello! Let's explore some concepts."
		fmt.Printf("💬 Initial Prompt: %s\n", promptText)
	} else {
		fmt.Printf("📖 Resuming exploration chat: %s (Active ThinkState: %s)\n", meta.ID, meta.ActiveThinkState)

		fmt.Print("\n> ")
		reader := bufio.NewReader(os.Stdin)
		input, readErr := reader.ReadString('\n')
		if readErr != nil && (!errors.Is(readErr, io.EOF) || input == "") {
			logger.Error("Failed reading prompt from stdin", "error", readErr)
			os.Exit(1)
		}
		promptText = strings.TrimSpace(input)

		if promptText == "" {
			fmt.Println("No input provided. Exiting.")
			return
		}
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

	userInterface := cli.NewTerminalUI()

	fmt.Println("\n🤖 Main Session Thinking...")

	if err := turnManager.RunTurn(ctx, *spaceName, meta.ID, promptText, userInterface, emitter); err != nil {
		logger.Error("Execution turn failed", "error", err)
		os.Exit(1)
	}

	fmt.Println("\nDone.")
}
