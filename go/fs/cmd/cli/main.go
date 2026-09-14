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

	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/cli"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/gitfs"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
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
	modelClient := llm.NewGenAIClient(client)

	homeDirectory, err := os.UserHomeDir()
	if err != nil {
		logger.Error("Failed to get user home directory", "error", err)
		os.Exit(1)
	}

	repositoryRoot := filepath.Join(homeDirectory, "Documents", "thinkspace", *spaceName)
	configurationsDirectory := filepath.Join(homeDirectory, "Documents", "thinkspace", "configs")

	if err := os.MkdirAll(repositoryRoot, 0755); err != nil {
		logger.Error("Failed to create workspace directory", "error", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(configurationsDirectory, 0755); err != nil {
		logger.Error("Failed to create configurations directory", "error", err)
		os.Exit(1)
	}

	// Ensure default templates exist before loading
	config.ScaffoldDefaults(logger, configurationsDirectory)

	var stateEngine workspace.ChatEngine
	switch *engineType {
	case "exec":
		fmt.Println("⚙️  Engine: Git CLI (ExecEngine with Worktrees)")
		stateEngine = gitfs.NewGoExecEngine(repositoryRoot, true)
	case "gogit":
		fallthrough
	default:
		fmt.Println("⚙️  Engine: go-git (GoGitEngine with Local Clones)")
		stateEngine = gitfs.NewGoGitEngine(repositoryRoot, true)
	}

	registry := config.NewRegistry()
	if err := registry.LoadDirectory(configurationsDirectory); err != nil {
		logger.Error("Failed to load configurations", "error", err)
		os.Exit(1)
	}

	activeThinkSpace, ok := registry.GetDomain(*domainName)
	if !ok {
		fmt.Printf("⚠️ Domain config '%s.yaml' not found in %s\n", *domainName, configurationsDirectory)
		os.Exit(1)
	}

	spaceConfiguration, _ := registry.GetConfig(*domainName)
	flowConfiguration, flowOk := registry.GetFlow("fanout")
	if !flowOk {
		logger.Error("FanOut configuration missing from registry")
		os.Exit(1)
	}

	// Wire Domain Dependencies
	bus := chat.NewEventBus()
	bus.Subscribe(chat.NewLedgerSubscriber())

	// FIXED: Requesting ModelCategoryWorker from the new spaces package
	workerModel := activeThinkSpace.Config().Models[spaces.ModelCategoryWorker]
	llmAdapter := llm.NewAdapter(modelClient)
	subAgentExecutor := llm.NewSubAgentExecutor(modelClient, workerModel, activeThinkSpace.Config().WorkerSystemPrompt(), activeThinkSpace.Config().MaxWorkerTokens)
	fanOutFlow := flows.NewFanOutFlow(logger)
	workspaceService := workspace.NewService(logger, stateEngine, repositoryRoot, bus)

	playbackEngine := chat.NewPlaybackEngine()
	contextAssembler := chat.NewContextAssembler()
	userInterface := cli.NewTerminalUI()

	// CLI doesn't use WebSockets, so it purely relies on the SlogEmitter
	emitter := flows.MultiFlowEmitter{flows.NewSlogEmitter(logger)}

	coordinator := session.NewCoordinator(
		logger,
		workspaceService,
		llmAdapter,
		subAgentExecutor,
		fanOutFlow,
		emitter,
		*domainName,
		spaceConfiguration.Roles.Worker,
		flowConfiguration,
	)

	fmt.Printf("📂 Workspace root: %s\n", repositoryRoot)
	fmt.Printf("📄 Active Domain: %s\n", *domainName)

	threadDirectory := filepath.Join(repositoryRoot, "chats", *chatName)
	isNewThread := false
	if _, err := os.Stat(threadDirectory); os.IsNotExist(err) {
		isNewThread = true
	}

	thread, err := workspaceService.StartThread(ctx, *chatName)
	if err != nil {
		logger.Error("Failed to start thread", "error", err)
		os.Exit(1)
	}

	if isNewThread {
		fmt.Printf("🌱 Creating new exploration thread: %s\n", *chatName)
		initialPrompt := "Generate a function to check if a point is in a unit circle. Fan this out to 2 agents using different mathematical approaches, and require unit tests."
		fmt.Printf("💬 Initial Prompt: %s\n", initialPrompt)

		_ = workspaceService.LogUserPrompt(ctx, thread, initialPrompt)
	} else {
		fmt.Printf("📖 Resuming exploration thread: %s\n", *chatName)

		graph, _, err := playbackEngine.LoadState(ctx, thread)
		if err == nil {
			fmt.Printf("Loaded %d historical turns.\n", len(graph.RecentEvents))
		}

		fmt.Print("\n> ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			fmt.Println("No input provided. Exiting.")
			return
		}

		_ = workspaceService.LogUserPrompt(ctx, thread, input)
	}

	// Build the assembled LLM history from the persistent state
	graph, manifest, err := playbackEngine.LoadState(ctx, thread)
	if err != nil {
		logger.Error("Failed to load playback state", "error", err)
		os.Exit(1)
	}

	assemblyReq := chat.ContextAssemblyRequest{
		Manifest:     manifest,
		ActiveLenses: []string{},
		RecentEvents: graph.RecentEvents,
	}

	history, err := contextAssembler.Build(assemblyReq)
	if err != nil {
		logger.Error("Failed to assemble context", "error", err)
		os.Exit(1)
	}

	fmt.Println("\n🤖 Main Session Thinking...")

	if err := coordinator.ExecuteTurn(ctx, thread, activeThinkSpace, history, userInterface); err != nil {
		logger.Error("Execution turn failed", "error", err)
		os.Exit(1)
	}

	fmt.Println("\nDone.")
}
