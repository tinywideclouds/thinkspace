package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/joho/godotenv"
	"google.golang.org/genai"
	"gopkg.in/yaml.v3"

	"github.com/tinywideclouds.com/thinkspace/internal/gitfs"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/net"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

func loadThinkSpaceConfig(path string) workspace.ThinkSpaceConfig {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("⚠️ Configuration not found at %s\n", path)
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
	port := flag.Int("port", 8080, "Port for the ThinkSpace WebServer")
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

	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Error("Failed to get home directory", "error", err)
		os.Exit(1)
	}

	repoRoot := filepath.Join(homeDir, "Documents", "thinkspace", *spaceName)
	configPath := filepath.Join(homeDir, "Documents", "thinkspace", "configs", "golang.yaml")

	_ = os.MkdirAll(repoRoot, 0755)

	var stateEngine workspace.StateEngine
	switch *engineType {
	case "exec":
		stateEngine = gitfs.NewGoExecEngine()
	default:
		stateEngine = gitfs.NewGoGitEngine()
	}

	tsConfig := loadThinkSpaceConfig(configPath)
	activeThinkSpace := workspace.NewGoThinkSpace(tsConfig)
	workerModel := activeThinkSpace.Model(workspace.ModelCategoryWorker)

	llmMgr := llm.NewManager(client)
	subAgentExecutor := llm.SubAgentFactory(client, workerModel)
	fanOutFlow := workspace.NewFanOutFlow("FanOut", logger)
	workspaceService := workspace.NewService(logger, stateEngine, repoRoot)
	coordinator := session.NewCoordinator(logger, workspaceService, llmMgr, subAgentExecutor, fanOutFlow)

	mux := http.NewServeMux()

	// WebSocket session endpoint
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true, // Allows dev connections from local frontend dev server
		})
		if err != nil {
			logger.Error("WebSocket upgrade failed", "error", err)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "session ended")

		wsCtx := r.Context()

		// PROPER PLUMBING: Create the token channel first
		tokenChan := make(chan workspace.AgentToken, 100)

		// Pass it into the WebSocketUI struct
		ui := net.NewWebSocketUI(wsCtx, conn, tokenChan)

		// Goroutine to multiplex sub-agent streams over the WebSocket
		go func() {
			for token := range tokenChan {
				_ = wsjson.Write(wsCtx, conn, net.WSEvent{
					Type: net.EventTypeAgentStream,
					Payload: mustMarshal(net.AgentStreamPayload{
						AgentID: token.AgentID,
						Text:    token.Text,
					}),
				})
			}
		}()

		// Inbound message loop (reads commands from browser)
		for {
			var event net.WSEvent
			err := wsjson.Read(wsCtx, conn, &event)
			if err != nil {
				logger.Info("WebSocket client disconnected")
				break
			}

			switch event.Type {
			case net.EventTypeSubmitPrompt:
				var payload net.SubmitPromptPayload
				if err := json.Unmarshal(event.Payload, &payload); err == nil {
					// Kick off the orchestration turn in background
					go func(promptText string) {
						chatName := "unit-circle-test"
						thread, err := workspaceService.StartThread(wsCtx, chatName)
						if err != nil {
							logger.Error("Failed to start thread", "error", err)
							return
						}

						_ = workspaceService.LogUserPrompt(wsCtx, thread, promptText)
						history := []*genai.Content{
							{Role: "user", Parts: []*genai.Part{{Text: promptText}}},
						}

						_ = coordinator.ExecuteTurn(wsCtx, thread, activeThinkSpace, history, ui)
					}(payload.Text)
				}

			case net.EventTypeSelectStrategy:
				var payload net.SelectStrategyPayload
				if err := json.Unmarshal(event.Payload, &payload); err == nil {
					ui.PushStrategy(payload.StrategyID)
				}

			case net.EventTypeReviewDecision:
				var payload net.ReviewDecisionPayload
				if err := json.Unmarshal(event.Payload, &payload); err == nil {
					ui.PushReviewDecision(payload.Accepted)
				}
			}
		}
	})

	serverAddr := fmt.Sprintf(":%d", *port)
	logger.Info("🚀 ThinkSpace Server listening", "addr", serverAddr)
	if err := http.ListenAndServe(serverAddr, mux); err != nil {
		logger.Error("Server stopped", "error", err)
	}
}

func mustMarshal(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
