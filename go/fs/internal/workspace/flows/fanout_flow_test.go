package flows_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/genai"

	"github.com/tinywideclouds.com/thinkspace/internal/gitfs"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace/flows"
)

type mockThinkSpace struct{}

func (m *mockThinkSpace) Name() string                                  { return "mock" }
func (m *mockThinkSpace) SystemPrompt() string                          { return "" }
func (m *mockThinkSpace) SubAgentSystemPrompt() string                  { return "" }
func (m *mockThinkSpace) Model(category workspace.ModelCategory) string { return "test-model" }
func (m *mockThinkSpace) Tools() []*genai.Tool                          { return nil }
func (m *mockThinkSpace) TurnTimeout() time.Duration                    { return 5 * time.Minute }
func (m *mockThinkSpace) AgentTimeout() time.Duration                   { return 1 * time.Minute }
func (m *mockThinkSpace) VerifyTimeout() time.Duration                  { return 15 * time.Second }

func (m *mockThinkSpace) Verify(ctx context.Context, sandbox workspace.CandidateSandbox) error {
	return nil
}

func TestFanOutFlow_Concurrency(t *testing.T) {
	// FAIL FAST(ish): 20-second timeout to prevent deadlocks
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	tempRoot := t.TempDir()

	stateEngine := gitfs.NewGoExecChat(tempRoot, true)
	svc := workspace.NewService(logger, stateEngine, tempRoot)

	thread, err := svc.StartThread(ctx, "stress-test-thread")
	if err != nil {
		t.Fatalf("failed to start thread: %v", err)
	}

	flow := flows.NewFanOutFlow("FanOut", logger)
	space := &mockThinkSpace{}

	tokenChan := make(chan workspace.AgentToken, 100)
	var generatedFiles int32

	mockExecutor := func(ctx context.Context, instructions string, sandbox workspace.CandidateSandbox, agentID int, tc chan<- workspace.AgentToken) error {
		if tc != nil {
			tc <- workspace.AgentToken{
				AgentID: agentID,
				Text:    fmt.Sprintf("Stream chunk from agent %d", agentID),
			}
		}

		fileName := fmt.Sprintf("code_%d.go", agentID)
		err := sandbox.WriteFile(ctx, fileName, []byte("package test"))
		if err == nil {
			atomic.AddInt32(&generatedFiles, 1)
		}
		return err
	}

	agentCount := 5
	args := map[string]any{
		"agent_count": float64(agentCount),
		"agent_instructions": []any{
			"Instruction 1",
			"Instruction 2",
			"Instruction 3",
			"Instruction 4",
			"Instruction 5",
		},
	}

	result, err := flow.Execute(ctx, svc, thread, space, args, mockExecutor, tokenChan)
	if err != nil {
		t.Fatalf("FanOutFlow failed: %v", err)
	}

	if len(result.Branches) != agentCount {
		t.Errorf("Expected %d successful branches to be submitted, got %d", agentCount, len(result.Branches))
	}

	close(tokenChan)
	tokenCount := 0
	agentTokensFound := make(map[int]bool)

	for token := range tokenChan {
		tokenCount++
		agentTokensFound[token.AgentID] = true
	}

	if tokenCount != agentCount {
		t.Errorf("Expected %d tokens in channel, got %d", agentCount, tokenCount)
	}

	if len(agentTokensFound) != agentCount {
		t.Errorf("Expected tokens from %d unique agents, got %d", agentCount, len(agentTokensFound))
	}

	if atomic.LoadInt32(&generatedFiles) != int32(agentCount) {
		t.Errorf("Expected %d files to be generated across sandboxes, got %d", agentCount, atomic.LoadInt32(&generatedFiles))
	}
}
