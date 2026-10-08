package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"google.golang.org/genai"

	"github.com/tinywideclouds.com/thinkspace/internal/cli"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/gitfs"
	"github.com/tinywideclouds.com/thinkspace/internal/llm/gemini"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

func main() {
	chatName := flag.String("chat", "unit-circle-test", "The name of the initial exploration thread")
	engineType := flag.String("engine", "gogit", "State engine backend to use ('gogit' or 'exec')")
	spaceName := flag.String("space", "sandbox", "The think space to use")
	domainName := flag.String("domain", "golang", "The domain configuration to load")
	flag.Parse()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	_ = godotenv.Load()

	client, err := genai.NewClient(ctx, nil)
	if err != nil {
		logger.Error("Failed to initialize GenAI client", "error", err)
		os.Exit(1)
	}
	modelClient := gemini.NewAdapter(client)

	homeDirectory, err := os.UserHomeDir()
	if err != nil {
		logger.Error("Failed to get user home directory", "error", err)
		os.Exit(1)
	}

	baseRoot := filepath.Join(homeDirectory, "Documents", "thinkspace")
	repositoryRoot := filepath.Join(baseRoot, *spaceName)
	configurationsDirectory := filepath.Join(baseRoot, "configs")

	if err := os.MkdirAll(repositoryRoot, 0755); err != nil {
		logger.Error("Failed to create workspace directory", "error", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(configurationsDirectory, 0755); err != nil {
		logger.Error("Failed to create configurations directory", "error", err)
		os.Exit(1)
	}

	config.ScaffoldDefaults(logger, configurationsDirectory)

	engineFactory := func(repoRoot string) workspace.ChatEngine {
		if *engineType == "exec" {
			return gitfs.NewGoExecEngine(repoRoot, true)
		}
		return gitfs.NewGoGitEngine(repoRoot, true)
	}

	serviceManager := workspace.NewServiceManager(logger, baseRoot, engineFactory)

	registry := config.NewRegistry()
	if err := registry.LoadDirectory(configurationsDirectory); err != nil {
		logger.Error("Failed to load configurations", "error", err)
		os.Exit(1)
	}

	_ = serviceManager.UpdateSpaceState(*spaceName, func(s *workspace.SpaceState) {
		s.Domain = *domainName
	})

	fmt.Printf("📂 Workspace root: %s\n", repositoryRoot)
	if *engineType == "exec" {
		fmt.Println("⚙️  Engine: Git CLI (ExecEngine with Worktrees)")
	} else {
		fmt.Println("⚙️  Engine: go-git (GoGitEngine with Local Clones)")
	}

	threadDirectory := filepath.Join(repositoryRoot, "chats", *chatName)
	isNewThread := false

	if _, err := os.Stat(threadDirectory); os.IsNotExist(err) {
		isNewThread = true
		_ = os.MkdirAll(threadDirectory, 0755)
		meta := workspace.ChatMeta{
			ID:           *chatName,
			Name:         *chatName,
			ActiveDomain: *domainName,
		}
		b, _ := json.MarshalIndent(meta, "", "  ")
		_ = os.WriteFile(filepath.Join(threadDirectory, "meta.json"), b, 0644)
	}

	var promptText string
	if isNewThread {
		fmt.Printf("🌱 Creating new exploration thread: %s (Domain: %s)\n", *chatName, *domainName)
		promptText = "Hello! Let's explore some concepts."
		fmt.Printf("💬 Initial Prompt: %s\n", promptText)
	} else {
		meta, _ := serviceManager.GetChatMeta(*spaceName, *chatName)
		fmt.Printf("📖 Resuming exploration thread: %s (Active Domain: %s)\n", *chatName, meta.ActiveDomain)

		fmt.Print("\n> ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		promptText = strings.TrimSpace(input)

		if promptText == "" {
			fmt.Println("No input provided. Exiting.")
			return
		}
	}

	// FLOW FACTORY INJECTION
	var fanOutCfg flows.FanOutConfig
	if err := registry.DecodeFlowConfig("fanout", &fanOutCfg); err != nil {
		logger.Warn("Failed to decode fanout config, using defaults", "error", err)
	}
	fanOutFlow := flows.NewFanOutFlow(logger, fanOutCfg)

	// CORE SYSTEM INJECTION
	var digestCfg session.DigestConfig
	if err := registry.DecodeCoreConfig("digest", &digestCfg); err != nil {
		logger.Warn("Failed to decode digest config, using defaults", "error", err)
	}
	digestManager := session.NewDigestManager(logger, modelClient, digestCfg)

	turnManager := session.NewTurnManager(logger, registry, serviceManager, modelClient, fanOutFlow, digestManager)

	userInterface := cli.NewTerminalUI()
	emitter := telemetry.MultiEmitter{telemetry.NewSlogEmitter(logger)}

	fmt.Println("\n🤖 Main Session Thinking...")

	if err := turnManager.RunTurn(ctx, *spaceName, *chatName, promptText, userInterface, emitter); err != nil {
		logger.Error("Execution turn failed", "error", err)
		os.Exit(1)
	}

	fmt.Println("\nDone.")
}
