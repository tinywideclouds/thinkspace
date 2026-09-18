package session_test

import (
	"context"
	"errors"
	"iter"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"google.golang.org/genai"
)

type mockDelegatorClient struct {
	responses []*genai.GenerateContentResponse
	callIndex int

	capturedPrompts []string // Stores the last message of every history submitted to the mock
}

func (m *mockDelegatorClient) GenerateContentStream(ctx context.Context, model string, history []*genai.Content, configuration *genai.GenerateContentConfig) iter.Seq2[*genai.GenerateContentResponse, error] {
	if len(history) > 0 && len(history[len(history)-1].Parts) > 0 {
		m.capturedPrompts = append(m.capturedPrompts, history[len(history)-1].Parts[0].Text)
	}

	return func(yield func(*genai.GenerateContentResponse, error) bool) {
		if m.callIndex < len(m.responses) {
			resp := m.responses[m.callIndex]
			m.callIndex++

			if resp == nil {
				yield(nil, errors.New("simulated stream failure"))
				return
			}
			yield(resp, nil)
		}
	}
}

func TestDelegator_ExecuteProposeChange_SkipStrategy(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()
	bus := chat.NewEventBus()

	workspaceService := workspace.NewService(logger, &mockChatEngine{}, workspaceRoot, bus)
	thread, _ := workspaceService.StartThread(ctx, "test-thread-skip")

	client := &mockDelegatorClient{}
	llmAdapter := llm.NewAdapter(client)
	flow := &mockFlow{}
	emitter := &mockEmitter{}

	delegator := session.NewDelegator(logger, workspaceService, llmAdapter, nil, flow, emitter, "golang", "", flows.FlowConfig{})
	ui := &mockUserInterface{strategyToReturn: session.StrategySkip}

	call := llm.ToolCall{
		Name: "propose_change",
		Args: map[string]any{},
	}

	err := delegator.ExecuteProposeChange(ctx, thread, &mockThinkSpace{}, nil, ui, call, "test-model")
	if err != nil {
		t.Fatalf("ExecuteProposeChange failed: %v", err)
	}

	if ui.reviewedBranch != "" {
		t.Errorf("Expected UI review step to be skipped, but got review for: %s", ui.reviewedBranch)
	}
}

func TestDelegator_ExecuteProposeChange_WrapUpStreamError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()
	bus := chat.NewEventBus()

	workspaceService := workspace.NewService(logger, &mockChatEngine{}, workspaceRoot, bus)
	thread, _ := workspaceService.StartThread(ctx, "test-thread-err")

	client := &mockDelegatorClient{
		responses: []*genai.GenerateContentResponse{
			nil, // Force the final wrapUpStream loop to fail
		},
	}
	llmAdapter := llm.NewAdapter(client)
	flow := &mockFlow{}
	emitter := &mockEmitter{}

	delegator := session.NewDelegator(logger, workspaceService, llmAdapter, nil, flow, emitter, "golang", "", flows.FlowConfig{})
	ui := &mockUserInterface{strategyToReturn: session.StrategyManual}

	call := llm.ToolCall{
		Name: "propose_change",
		Args: map[string]any{},
	}

	err := delegator.ExecuteProposeChange(ctx, thread, &mockThinkSpace{}, nil, ui, call, "test-model")

	if err != nil {
		t.Fatalf("Expected delegator to swallow wrap-up stream error and complete gracefully, but it returned: %v", err)
	}
}

func TestDelegator_TelemetryEmission(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()
	bus := chat.NewEventBus()

	workspaceService := workspace.NewService(logger, &mockChatEngine{}, workspaceRoot, bus)
	thread, _ := workspaceService.StartThread(ctx, "test-thread-telemetry")

	client := &mockDelegatorClient{
		responses: []*genai.GenerateContentResponse{},
	}

	emitter := &mockEmitter{}
	delegator := session.NewDelegator(logger, workspaceService, llm.NewAdapter(client), nil, &mockFlow{}, emitter, "golang", "", flows.FlowConfig{})

	ui := &mockUserInterface{strategyToReturn: session.StrategyManual}
	call := llm.ToolCall{Name: "propose_change", Args: map[string]any{}}

	_ = delegator.ExecuteProposeChange(ctx, thread, &mockThinkSpace{}, nil, ui, call, "test-model")

	var foundPropose, foundStrategy bool
	for _, e := range emitter.events {
		if e.channel != "manager" {
			t.Errorf("Expected Delegator to only emit to 'manager' channel")
		}

		if e.event.Type == "log" && strings.Contains(e.event.Message, "Initiating parallel FanOut Flow") {
			foundPropose = true
		}

		if e.event.Type == "log" && strings.Contains(e.event.Message, "User selected strategy") {
			foundStrategy = true
			if e.event.Payload["strategy"] != "Manual Review" {
				t.Errorf("Expected strategy payload to be 'Manual Review', got %v", e.event.Payload["strategy"])
			}
		}
	}

	if !foundPropose {
		t.Errorf("Expected Delegator to emit fan-out initiation log event")
	}
	if !foundStrategy {
		t.Errorf("Expected Delegator to emit strategy selection log event with payload")
	}
}

func TestDelegator_ExecuteProposeChange_ReviewTemplate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()
	bus := chat.NewEventBus()

	workspaceService := workspace.NewService(logger, &mockChatEngine{}, workspaceRoot, bus)
	thread, _ := workspaceService.StartThread(ctx, "test-thread-template")

	client := &mockDelegatorClient{
		responses: []*genai.GenerateContentResponse{
			{
				Candidates: []*genai.Candidate{{
					Content: &genai.Content{
						Parts: []*genai.Part{{Text: "Mock LLM Evaluation"}},
					},
				}},
			},
			{
				Candidates: []*genai.Candidate{{
					Content: &genai.Content{
						Parts: []*genai.Part{{Text: "Wrap up"}},
					},
				}},
			},
		},
	}

	llmAdapter := llm.NewAdapter(client)
	flow := &mockFlow{}
	emitter := &mockEmitter{}

	// Inject our custom YAML-driven templates
	config := flows.FlowConfig{
		ReviewPrompt: "CUSTOM REVIEW:\n{{ range .Branches }}Branch: {{ .CandidateID }} | Passed: {{ .Passed }}\n{{ end }}",
	}

	delegator := session.NewDelegator(logger, workspaceService, llmAdapter, nil, flow, emitter, "golang", "", config)

	ui := &mockUserInterface{
		strategyToReturn: session.StrategyReview,
		reviewToReturn:   true,
	}

	call := llm.ToolCall{
		Name: "propose_change",
		Args: map[string]any{},
	}

	err := delegator.ExecuteProposeChange(ctx, thread, &mockThinkSpace{}, nil, ui, call, "test-model")
	if err != nil {
		t.Fatalf("ExecuteProposeChange failed: %v", err)
	}

	if len(client.capturedPrompts) == 0 {
		t.Fatalf("Expected history to be captured by the mock LLM client")
	}

	expectedOutput := "CUSTOM REVIEW:\nBranch: candidate/mock-123 | Passed: true\n"
	found := false
	for _, prompt := range client.capturedPrompts {
		if strings.Contains(prompt, expectedOutput) {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Template execution failed. Expected prompt to contain:\n%s\nGot prompts:\n%v", expectedOutput, client.capturedPrompts)
	}
}
