package main

import (
	"bufio"
	"context"
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
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace/flows"
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
	modelClient := llm.NewGenAIClient(client)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Error("Failed to get user home directory", "error", err)
		os.Exit(1)
	}

	repoRoot := filepath.Join(homeDir, "Documents", "thinkspace", *spaceName)
	configsDir := filepath.Join(homeDir, "Documents", "thinkspace", "configs")

	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		logger.Error("Failed to create workspace directory", "error", err)
		os.Exit(1)
	}

	// FIXED: Using the new ChatEngine interface and stateful constructors
	var stateEngine workspace.ChatEngine
	switch *engineType {
	case "exec":
		fmt.Println("⚙️  Engine: Git CLI (ExecEngine with Worktrees)")
		stateEngine = gitfs.NewGoExecChat(repoRoot, true)
	case "gogit":
		fallthrough
	default:
		fmt.Println("⚙️  Engine: go-git (GoGitEngine with Local Clones)")
		stateEngine = gitfs.NewGoGitChat(repoRoot, true)
	}

	// 1. Initialize Registry and dynamically load the requested domain
	registry := config.NewRegistry()
	if err := registry.LoadDirectory(configsDir); err != nil {
		logger.Error("Failed to load configs", "error", err)
		os.Exit(1)
	}

	activeThinkSpace, ok := registry.GetSpace(*domainName)
	if !ok {
		fmt.Printf("⚠️ Domain config '%s.yaml' not found in %s\n", *domainName, configsDir)
		os.Exit(1)
	}

	workerModel := activeThinkSpace.Model(workspace.ModelCategoryWorker)
	llmMgr := llm.NewManager(modelClient)
	subAgentExecutor := llm.SubAgentFactory(modelClient, workerModel)
	fanOutFlow := flows.NewFanOutFlow("FanOut", logger)
	workspaceService := workspace.NewService(logger, stateEngine, repoRoot)

	// Inject the newly extracted Terminal UI
	ui := cli.NewTerminalUI()
	coordinator := session.NewCoordinator(logger, workspaceService, llmMgr, subAgentExecutor, fanOutFlow)

	fmt.Printf("📂 Workspace root: %s\n", repoRoot)
	fmt.Printf("📄 Active Domain: %s\n", *domainName)

	threadDir := filepath.Join(repoRoot, "chats", *chatName)
	ledgerPath := filepath.Join(threadDir, "conversation.jsonl")
	var thread *workspace.Thread
	var history []*genai.Content

	if _, err := os.Stat(threadDir); os.IsNotExist(err) {
		fmt.Printf("🌱 Creating new exploration thread: %s\n", *chatName)
		thread, err = workspaceService.StartThread(ctx, *chatName)
		if err != nil {
			logger.Error("Failed to start thread", "error", err)
			os.Exit(1)
		}

		initialPrompt := "Generate a function to check if a point is in a unit circle. Fan this out to 2 agents using different mathematical approaches, and require unit tests."
		fmt.Printf("💬 Initial Prompt: %s\n", initialPrompt)

		_ = workspaceService.LogUserPrompt(ctx, thread, initialPrompt)
		history = append(history, &genai.Content{Role: "user", Parts: []*genai.Part{{Text: initialPrompt}}})

	} else {
		fmt.Printf("📖 Resuming exploration thread: %s\n", *chatName)
		thread = &workspace.Thread{
			ID:         *chatName,
			Branch:     fmt.Sprintf("chat/%s", *chatName),
			LedgerPath: ledgerPath,
			Dir:        threadDir,
		}

		events, err := workspaceService.LoadEvents(ctx, thread)
		if err != nil {
			logger.Error("Failed to load history", "error", err)
			os.Exit(1)
		}

		history = llmMgr.BuildHistory(events)
		fmt.Printf("Loaded %d historical turns.\n", len(history))

		fmt.Print("\n> ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			fmt.Println("No input provided. Exiting.")
			return
		}

		_ = workspaceService.LogUserPrompt(ctx, thread, input)
		history = append(history, &genai.Content{Role: "user", Parts: []*genai.Part{{Text: input}}})
	}

	fmt.Println("\n🤖 Main Session Thinking...")

	if err := coordinator.ExecuteTurn(ctx, thread, activeThinkSpace, history, ui); err != nil {
		logger.Error("Execution turn failed", "error", err)
		os.Exit(1)
	}

	fmt.Println("\nDone.")
}
