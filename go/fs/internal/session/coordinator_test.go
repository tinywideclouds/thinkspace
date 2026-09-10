package session_test

import (
	"context"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"google.golang.org/genai"
)

// --- Mocks ---

type mockModelClient struct {
	responses []*genai.GenerateContentResponse
}

func (m *mockModelClient) GenerateContentStream(ctx context.Context, model string, history []*genai.Content, configuration *genai.GenerateContentConfig) iter.Seq2[*genai.GenerateContentResponse, error] {
	return func(yield func(*genai.GenerateContentResponse, error) bool) {
		for _, response := range m.responses {
			if !yield(response, nil) {
				return
			}
		}
	}
}

type mockChatEngine struct{}

func (m *mockChatEngine) InitChat(ctx context.Context, chatID string) error { return nil }
func (m *mockChatEngine) Snapshot(ctx context.Context, chatID string, message string) (string, error) {
	return "mock-sha", nil
}
func (m *mockChatEngine) SpawnCandidateSandbox(ctx context.Context, chatID string, candidateID string) (workspace.CandidateSandbox, error) {
	return nil, nil
}
func (m *mockChatEngine) PreviewCandidate(ctx context.Context, chatID string, candidateID string) error {
	return nil
}
func (m *mockChatEngine) Accept(ctx context.Context, chatID string, candidateID string, reason string) error {
	return nil
}
func (m *mockChatEngine) Reject(ctx context.Context, chatID string, candidateID string, reason string) error {
	return nil
}
func (m *mockChatEngine) ReadCandidateDiff(ctx context.Context, chatID, candidateID string) (string, error) {
	return "+ mock diff", nil
}

type mockThinkSpace struct{}

func (m *mockThinkSpace) Config() workspace.ThinkSpaceConfig {
	cfg := workspace.ThinkSpaceConfig{
		Models: map[workspace.ModelCategory]string{
			workspace.ModelCategoryManager: "test-manager",
		},
		TurnTimeoutSeconds: 300,
	}
	cfg.ApplyDefaults()
	return cfg
}

func (m *mockThinkSpace) Tools() []*genai.Tool         { return nil }
func (m *mockThinkSpace) Verifier() workspace.Verifier { return &mockVerifier{} }

type mockVerifier struct{}

func (m *mockVerifier) Verify(ctx context.Context, sandbox workspace.CandidateSandbox) error {
	return nil
}

type mockFlow struct {
	executeCalled bool
}

func (m *mockFlow) Name() string { return "MockFlow" }
func (m *mockFlow) Execute(ctx context.Context, workspaceService *workspace.Service, thread *chat.Thread, space workspace.ThinkSpace, arguments map[string]any, flowConfiguration flows.FlowConfig, flowContext flows.FlowContext, emitter flows.FlowEmitter, executor workspace.SubAgentExecutor, verifier workspace.Verifier) (*flows.FlowResult, error) {
	m.executeCalled = true
	return &flows.FlowResult{
		Branches: []string{"candidate/mock-123"},
		Summary:  "Mock delegation complete",
	}, nil
}

type mockUserInterface struct {
	strategyToReturn session.DelegationStrategy
	reviewToReturn   bool
	reviewedBranch   string
}

func (u *mockUserInterface) OnTextChunk(text string) {}

func (u *mockUserInterface) ChooseNextStep() session.DelegationStrategy {
	return u.strategyToReturn
}

func (u *mockUserInterface) ReviewCandidate(branch string) bool {
	u.reviewedBranch = branch
	return u.reviewToReturn
}

type mockEmitter struct{}

func (m *mockEmitter) Emit(event flows.FlowEvent) {}

// --- Tests ---

func TestCoordinator_ExecuteTurn_WithToolCall(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	workspaceRoot := t.TempDir()

	bus := chat.NewEventBus()
	workspaceService := workspace.NewService(logger, &mockChatEngine{}, workspaceRoot, bus)
	thread, _ := workspaceService.StartThread(ctx, "test-thread")

	os.MkdirAll(filepath.Join(workspaceRoot, "chats", "test-thread"), 0755)

	client := &mockModelClient{
		responses: []*genai.GenerateContentResponse{
			{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []*genai.Part{
								{Text: "I will implement that now."},
								{
									FunctionCall: &genai.FunctionCall{
										Name: "propose_change",
										Args: map[string]any{
											"agent_count": float64(2),
											"agent_tasks": []any{
												map[string]any{"context_digest": "Context A", "instruction": "Do task A"},
												map[string]any{"context_digest": "Context B", "instruction": "Do task B"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	llmAdapter := llm.NewAdapter(client)
	flow := &mockFlow{}
	executor := func(ctx context.Context, briefing workspace.SubAgentBriefing, sandbox workspace.CandidateSandbox, agentID int, tokenChannel chan<- workspace.AgentToken) error {
		return nil
	}

	emitter := &mockEmitter{}
	flowConfiguration := flows.FlowConfig{RetryPrompt: "retry"}

	coordinator := session.NewCoordinator(logger, workspaceService, llmAdapter, executor, flow, emitter, "golang", "use /src", flowConfiguration)

	userInterface := &mockUserInterface{
		strategyToReturn: session.StrategyManual,
		reviewToReturn:   true,
	}

	space := &mockThinkSpace{}
	var history []*genai.Content

	err := coordinator.ExecuteTurn(ctx, thread, space, history, userInterface)
	if err != nil {
		t.Fatalf("ExecuteTurn failed: %v", err)
	}

	if !flow.executeCalled {
		t.Errorf("Expected Flow to be executed upon intercepting tool call")
	}

	if userInterface.reviewedBranch != "candidate/mock-123" {
		t.Errorf("Expected UI to review 'candidate/mock-123', got '%s'", userInterface.reviewedBranch)
	}
}

func TestCoordinator_ExecuteTurn_SkipStrategy(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	workspaceRoot := t.TempDir()

	bus := chat.NewEventBus()
	workspaceService := workspace.NewService(logger, &mockChatEngine{}, workspaceRoot, bus)
	thread, _ := workspaceService.StartThread(ctx, "test-thread-skip")

	client := &mockModelClient{
		responses: []*genai.GenerateContentResponse{
			{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []*genai.Part{
								{
									FunctionCall: &genai.FunctionCall{
										Name: "propose_change",
										Args: map[string]any{
											"agent_count": float64(1),
											"agent_tasks": []any{
												map[string]any{"context_digest": "Skip context", "instruction": "Test skip"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	emitter := &mockEmitter{}
	flowConfiguration := flows.FlowConfig{}
	coordinator := session.NewCoordinator(logger, workspaceService, llm.NewAdapter(client), nil, &mockFlow{}, emitter, "golang", "", flowConfiguration)

	userInterface := &mockUserInterface{
		strategyToReturn: session.StrategySkip,
	}

	err := coordinator.ExecuteTurn(ctx, thread, &mockThinkSpace{}, nil, userInterface)
	if err != nil {
		t.Fatalf("ExecuteTurn failed: %v", err)
	}

	if userInterface.reviewedBranch != "" {
		t.Errorf("Expected UI review step to be skipped, but got review for: %s", userInterface.reviewedBranch)
	}
}
