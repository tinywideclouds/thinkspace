package session_test

import (
	"context"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"google.golang.org/genai"
)

// --- Mocks ---

type mockModelClient struct {
	responses []*genai.GenerateContentResponse
}

func (m *mockModelClient) GenerateContentStream(ctx context.Context, model string, history []*genai.Content, config *genai.GenerateContentConfig) iter.Seq2[*genai.GenerateContentResponse, error] {
	return func(yield func(*genai.GenerateContentResponse, error) bool) {
		for _, resp := range m.responses {
			if !yield(resp, nil) {
				return
			}
		}
	}
}

type mockStateEngine struct{}

func (m *mockStateEngine) InitThread(ctx context.Context, mainDir string, threadID string) error {
	return nil
}
func (m *mockStateEngine) Snapshot(ctx context.Context, mainDir string, threadID string, message string) (string, error) {
	return "mock-sha", nil
}
func (m *mockStateEngine) SpawnSandbox(ctx context.Context, mainDir string, threadID string, candidateID string) (string, error) {
	return "", nil
}
func (m *mockStateEngine) CommitSandbox(ctx context.Context, sandboxDir string, message string) (string, error) {
	return "mock-sha", nil
}
func (m *mockStateEngine) SubmitSandbox(ctx context.Context, sandboxDir string, mainDir string, candidateID string) error {
	return nil
}
func (m *mockStateEngine) CloseSandbox(ctx context.Context, sandboxDir string) error { return nil }
func (m *mockStateEngine) PreviewCandidate(ctx context.Context, mainDir string, threadID string, candidateID string) error {
	return nil
}
func (m *mockStateEngine) Accept(ctx context.Context, mainDir string, threadID string, candidateID string, reason string) error {
	return nil
}
func (m *mockStateEngine) Reject(ctx context.Context, mainDir string, threadID string, candidateID string, reason string) error {
	return nil
}
func (m *mockStateEngine) ReadCandidateDiff(ctx context.Context, repoRoot, threadID, candidateID string) (string, error) {
	return "+ mock diff", nil
}

type mockThinkSpace struct{}

func (m *mockThinkSpace) Name() string                                  { return "mock" }
func (m *mockThinkSpace) SystemPrompt() string                          { return "system prompt" }
func (m *mockThinkSpace) SubAgentSystemPrompt() string                  { return "sub agent prompt" }
func (m *mockThinkSpace) Model(category workspace.ModelCategory) string { return "test-model" }
func (m *mockThinkSpace) Tools() []*genai.Tool                          { return nil }
func (m *mockThinkSpace) TurnTimeout() time.Duration                    { return 5 * time.Minute }
func (m *mockThinkSpace) AgentTimeout() time.Duration                   { return 1 * time.Minute }
func (m *mockThinkSpace) VerifyTimeout() time.Duration                  { return 15 * time.Second }
func (m *mockThinkSpace) Verify(ctx context.Context, dir string) error  { return nil }

type mockFlow struct {
	executeCalled bool
}

func (m *mockFlow) Name() string { return "MockFlow" }
func (m *mockFlow) Execute(ctx context.Context, svc *workspace.Service, thread *workspace.Thread, space workspace.ThinkSpace, args map[string]any, executor workspace.SubAgentExecutor, tokenChan chan<- workspace.AgentToken) (*workspace.FlowResult, error) {
	m.executeCalled = true
	return &workspace.FlowResult{
		Branches: []string{"candidate/mock-123"},
		Summary:  "Mock delegation complete",
	}, nil
}

type mockUI struct {
	strategyToReturn    session.DelegationStrategy
	reviewToReturn      bool
	delegationStarted   bool
	delegationCompleted bool
	reviewedBranch      string
}

func (u *mockUI) OnTextChunk(text string) {}
func (u *mockUI) OnDelegationStart(agentCount int, instructions string) {
	u.delegationStarted = true
}
func (u *mockUI) OnDelegationComplete(summary string) {
	u.delegationCompleted = true
}
func (u *mockUI) ChooseNextStep() session.DelegationStrategy {
	return u.strategyToReturn
}
func (u *mockUI) ReviewCandidate(branch string) bool {
	u.reviewedBranch = branch
	return u.reviewToReturn
}
func (u *mockUI) GetAgentTokenChannel() chan<- workspace.AgentToken {
	return nil
}

// --- Tests ---

func TestCoordinator_ExecuteTurn_WithToolCall(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	workspaceRoot := t.TempDir()
	svc := workspace.NewService(logger, &mockStateEngine{}, workspaceRoot)
	thread, _ := svc.StartThread(ctx, "test-thread")

	// Create the thread directories so the ledger can be saved
	os.MkdirAll(filepath.Join(workspaceRoot, "chats", "test-thread"), 0755)

	// Simulate an LLM response containing a tool call
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
											"agent_count":        float64(2),
											"agent_instructions": []any{"Do task A", "Do task B"},
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

	llmMgr := llm.NewManager(client)
	flow := &mockFlow{}
	executor := func(ctx context.Context, instructions string, sandboxDir string, agentID int, tokenChan chan<- workspace.AgentToken) error {
		return nil
	}

	coordinator := session.NewCoordinator(logger, svc, llmMgr, executor, flow)

	// Script the UI to pick StrategyManual and accept the candidate
	ui := &mockUI{
		strategyToReturn: session.StrategyManual,
		reviewToReturn:   true,
	}

	space := &mockThinkSpace{}
	var history []*genai.Content

	err := coordinator.ExecuteTurn(ctx, thread, space, history, ui)
	if err != nil {
		t.Fatalf("ExecuteTurn failed: %v", err)
	}

	if !flow.executeCalled {
		t.Errorf("Expected Flow to be executed upon intercepting tool call")
	}

	if !ui.delegationStarted {
		t.Errorf("Expected UI.OnDelegationStart to be called")
	}

	if !ui.delegationCompleted {
		t.Errorf("Expected UI.OnDelegationComplete to be called")
	}

	if ui.reviewedBranch != "candidate/mock-123" {
		t.Errorf("Expected UI to review 'candidate/mock-123', got '%s'", ui.reviewedBranch)
	}
}

func TestCoordinator_ExecuteTurn_SkipStrategy(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	workspaceRoot := t.TempDir()
	svc := workspace.NewService(logger, &mockStateEngine{}, workspaceRoot)
	thread, _ := svc.StartThread(ctx, "test-thread-skip")

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
											"agent_count":        float64(1),
											"agent_instructions": []any{"Test skip"},
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

	coordinator := session.NewCoordinator(logger, svc, llm.NewManager(client), nil, &mockFlow{})

	// Script the UI to skip review entirely
	ui := &mockUI{
		strategyToReturn: session.StrategySkip,
	}

	err := coordinator.ExecuteTurn(ctx, thread, &mockThinkSpace{}, nil, ui)
	if err != nil {
		t.Fatalf("ExecuteTurn failed: %v", err)
	}

	// Because we chose Skip, the UI should never be asked to review a specific candidate
	if ui.reviewedBranch != "" {
		t.Errorf("Expected UI review step to be skipped, but got review for: %s", ui.reviewedBranch)
	}
}
