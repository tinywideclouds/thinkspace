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
	"gopkg.in/yaml.v3"

	"github.com/tinywideclouds.com/thinkspace/internal/gitfs"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// TerminalUI implements session.UserInterface for the CLI binary.
type TerminalUI struct {
	reader *bufio.Reader
}

func (ui *TerminalUI) OnTextChunk(text string) {
	fmt.Print(text)
}

func (ui *TerminalUI) OnDelegationStart(count int, instructions string) {
	fmt.Printf("\n\n🚀 Delegating task to %d agent(s): %s\n", count, instructions)
}

func (ui *TerminalUI) OnDelegationComplete(summary string) {
	fmt.Println("\n✅ Delegation Flow Complete:\n" + summary)
}

func (ui *TerminalUI) ChooseNextStep() session.DelegationStrategy {
	fmt.Println("\nSelect Next Step:")
	fmt.Println("[1] Manual Review (I will read the generated code)")
	fmt.Println("[2] Assisted Review (Manager evaluates diffs, I decide)")
	fmt.Println("[3] Auto-Refine (Manager evaluates, synthesizes a final branch, I approve)")
	fmt.Println("[0] Skip / Abort (Reject all and continue)")
	fmt.Print("Choice [1]: ")

	input, _ := ui.reader.ReadString('\n')
	input = strings.TrimSpace(input)

	switch input {
	case "0":
		return session.StrategySkip
	case "2":
		return session.StrategyReview
	case "3":
		return session.StrategyRefine
	default:
		return session.StrategyManual
	}
}

func (ui *TerminalUI) ReviewCandidate(branch string) bool {
	fmt.Printf("\n👀 Previewing %s...\n", branch)
	fmt.Println("--------------------------------------------------")
	fmt.Printf("📂 Files have been successfully checked out.\n")
	fmt.Printf("💻 Open your IDE to inspect the code for %s.\n", branch)
	fmt.Println("--------------------------------------------------")

	fmt.Print("Accept candidate? (y/n): ")
	accept, _ := ui.reader.ReadString('\n')
	return strings.ToLower(strings.TrimSpace(accept)) == "y"
}

func (ui *TerminalUI) GetAgentTokenChannel() chan<- workspace.AgentToken {
	// The CLI doesn't multiplex agent tokens, so we return nil.
	// The SubAgentExecutor handles this safely.
	return nil
}

func loadThinkSpaceConfig(path string) workspace.ThinkSpaceConfig {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("⚠️ Configuration not found at %s. Application requires a domain configuration to start.\n", path)
		os.Exit(1)
	}
	var cfg workspace.ThinkSpaceConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		fmt.Printf("⚠️ Invalid YAML at %s: %v\n", path, err)
		os.Exit(1)
	}
	cfg.ApplyDefaults()
	return cfg
}

func main() {
	chatName := flag.String("chat", "unit-circle-test", "The name of the exploration thread")
	engineType := flag.String("engine", "gogit", "State engine backend to use ('gogit' or 'exec')")
	spaceName := flag.String("space", "sandbox", "The think space to use")
	flag.Parse()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	_ = godotenv.Load()

	client, err := genai.NewClient(ctx, nil)
	if err != nil {
		logger.Error("Failed to initialize GenAI client", "error", err)
		os.Exit(1)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Error("Failed to get user home directory", "error", err)
		os.Exit(1)
	}

	repoRoot := filepath.Join(homeDir, "Documents", "thinkspace", *spaceName)
	configPath := filepath.Join(homeDir, "Documents", "thinkspace", "configs", "golang.yaml")

	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		logger.Error("Failed to create workspace directory", "error", err)
		os.Exit(1)
	}

	var stateEngine workspace.StateEngine
	switch *engineType {
	case "exec":
		fmt.Println("⚙️  Engine: Git CLI (ExecEngine with Worktrees)")
		stateEngine = gitfs.NewGoExecEngine()
	case "gogit":
		fallthrough
	default:
		fmt.Println("⚙️  Engine: go-git (GoGitEngine with Local Clones)")
		stateEngine = gitfs.NewGoGitEngine()
	}

	tsConfig := loadThinkSpaceConfig(configPath)
	activeThinkSpace := workspace.NewGoThinkSpace(tsConfig)
	workerModel := activeThinkSpace.Model(workspace.ModelCategoryWorker)

	llmMgr := llm.NewManager(client)
	subAgentExecutor := llm.SubAgentFactory(client, workerModel)
	fanOutFlow := workspace.NewFanOutFlow("FanOut", logger)
	workspaceService := workspace.NewService(logger, stateEngine, repoRoot)

	ui := &TerminalUI{reader: bufio.NewReader(os.Stdin)}
	coordinator := session.NewCoordinator(logger, workspaceService, llmMgr, subAgentExecutor, fanOutFlow)

	fmt.Printf("📂 Workspace root: %s\n", repoRoot)
	fmt.Printf("📄 Config loaded from: %s\n", configPath)

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
		input, _ := ui.reader.ReadString('\n')
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
