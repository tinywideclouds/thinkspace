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

	"github.com/tinywideclouds.com/thinkspace/internal/gitfs"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"

	"google.golang.org/genai"
)

func main() {
	chatName := flag.String("chat", "concept-test", "The name of the exploration thread")
	flag.Parse()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	_ = godotenv.Load()

	client, err := genai.NewClient(ctx, nil)
	if err != nil {
		logger.Error("Failed to initialize GenAI client", "error", err)
		os.Exit(1)
	}

	stateEngine := gitfs.NewNativeEngine()
	llmMgr := llm.NewManager(client)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Error("Failed to get user home directory", "error", err)
		os.Exit(1)
	}

	repoRoot := filepath.Join(homeDir, "Documents", "thinkspace", "sandboxB")
	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		logger.Error("Failed to create workspace directory", "error", err)
		os.Exit(1)
	}

	fmt.Printf("📂 Using workspace root: %s\n", repoRoot)

	svc := workspace.NewService(logger, stateEngine, repoRoot)

	var thread *workspace.Thread
	var history []*genai.Content

	threadDir := filepath.Join(repoRoot, "chats", *chatName)
	ledgerPath := filepath.Join(threadDir, "conversation.jsonl")

	if _, err := os.Stat(threadDir); os.IsNotExist(err) {
		fmt.Printf("🌱 Creating new exploration thread: %s\n", *chatName)
		thread, err = svc.StartThread(ctx, *chatName)
		if err != nil {
			logger.Error("Failed to start thread", "error", err)
			os.Exit(1)
		}

		initialPrompt := "Create a file called `math.go` with a function that adds two integers."
		fmt.Printf("💬 Initial Prompt: %s\n", initialPrompt)

		if err := svc.LogUserPrompt(ctx, thread, initialPrompt); err != nil {
			logger.Error("Failed to log prompt", "error", err)
		}
		history = append(history, &genai.Content{Role: "user", Parts: []*genai.Part{{Text: initialPrompt}}})

	} else {
		fmt.Printf("📖 Resuming exploration thread: %s\n", *chatName)
		thread = &workspace.Thread{
			ID:         *chatName,
			Branch:     fmt.Sprintf("chat/%s", *chatName),
			LedgerPath: ledgerPath,
			Dir:        threadDir,
		}

		events, err := svc.LoadEvents(ctx, thread)
		if err != nil {
			logger.Error("Failed to load history from ledger", "error", err)
			os.Exit(1)
		}

		history = llmMgr.BuildHistory(events)
		fmt.Printf("Loaded %d historical turns from ledger.\n", len(history))

		fmt.Print("\n> ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			fmt.Println("No input provided. Exiting.")
			return
		}

		if err := svc.LogUserPrompt(ctx, thread, input); err != nil {
			logger.Error("Failed to log prompt", "error", err)
		}
		history = append(history, &genai.Content{Role: "user", Parts: []*genai.Part{{Text: input}}})
	}

	fmt.Println("\n🤖 Generating...")

	stream := llmMgr.GenerateStream(ctx, "gemini-3.5-flash", history)

	var fullModelResponse strings.Builder
	var interceptedTools []llm.ToolCall

	for chunk, err := range stream {
		if err != nil {
			logger.Error("Stream generation failed", "error", err)
			os.Exit(1)
		}

		if len(chunk.Candidates) > 0 && chunk.Candidates[0].Content != nil {
			for _, part := range chunk.Candidates[0].Content.Parts {
				if part.Text != "" {
					fmt.Print(part.Text)
					fullModelResponse.WriteString(part.Text)
				}
			}
		}

		calls := llmMgr.InterceptToolCalls(chunk)
		interceptedTools = append(interceptedTools, calls...)
	}

	fmt.Println()

	if fullModelResponse.Len() > 0 {
		if err := svc.LogModelResponse(ctx, thread, fullModelResponse.String()); err != nil {
			logger.Error("Failed to log model response", "error", err)
		}
	}

	for _, call := range interceptedTools {
		fmt.Printf("\n🛠️  Tool Call Detected: Proposing %s\n", call.FilePath)
		fmt.Printf("   Reasoning: %s\n", call.Reasoning)

		files := map[string][]byte{
			call.FilePath: []byte(call.NewContent + call.Patch),
		}

		candidate, err := svc.ProposeCandidate(ctx, thread, "propose_change", files)
		if err != nil {
			logger.Error("Failed to propose candidate", "error", err)
			continue
		}

		// RESTORED: Tell the user exactly what is happening
		fmt.Printf("   👀 Candidate branch '%s' is checked out and ready for inspection.\n", candidate.Branch)
		fmt.Print("   Inspect the files in your editor. Accept candidate? (y/n): ")

		reader := bufio.NewReader(os.Stdin)
		decision, _ := reader.ReadString('\n')
		accept := strings.ToLower(strings.TrimSpace(decision)) == "y"

		if err := svc.ResolveCandidate(ctx, thread, candidate, accept); err != nil {
			logger.Error("Resolution failed", "error", err)
		}

		if accept {
			fmt.Println("   ✅ Candidate accepted and merged into the Thread.")
		} else {
			fmt.Println("   ❌ Candidate rejected. Thread remains unmodified.")
		}
	}

	fmt.Println("\n💾 Checkpointing ledger...")
	if _, err := svc.Checkpoint(ctx, thread, "End of CLI turn"); err != nil {
		logger.Error("Failed to checkpoint ledger", "error", err)
	}
	fmt.Println("Done.")
}
