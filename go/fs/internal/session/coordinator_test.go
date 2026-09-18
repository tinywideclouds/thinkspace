package session_test

import (
	"context"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/assembler"
	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"google.golang.org/genai"
)

// --- Mocks ---

type mockModelClient struct {
	responses []*genai.GenerateContentResponse
	callIndex int
}

func (m *mockModelClient) GenerateContentStream(ctx context.Context, model string, history []*genai.Content, configuration *genai.GenerateContentConfig) iter.Seq2[*genai.GenerateContentResponse, error] {
	return func(yield func(*genai.GenerateContentResponse, error) bool) {
		if m.callIndex < len(m.responses) {
			resp := m.responses[m.callIndex]
			m.callIndex++
			yield(resp, nil)
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

func (m *mockThinkSpace) Config() spaces.ThinkSpaceConfig {
	cfg := spaces.ThinkSpaceConfig{
		Roles: spaces.RolesConfig{
			Manager: spaces.ManagerConfig{Model: "test-manager"},
			Worker:  spaces.WorkerConfig{Model: "test-worker"},
		},
		Timeouts: spaces.TimeoutsConfig{TurnSeconds: 300},
	}
	cfg.ApplyDefaults()
	return cfg
}
func (m *mockThinkSpace) Tools() []*genai.Tool       { return nil }
func (m *mockThinkSpace) Verifier() spaces.Verifier  { return &mockVerifier{} }
func (m *mockThinkSpace) Mapbook() assembler.Mapbook { return nil }
func (m *mockThinkSpace) Patcher() workspace.Patcher { return nil }

type mockVerifier struct{}

func (m *mockVerifier) Verify(ctx context.Context, sandbox workspace.CandidateSandbox) error {
	return nil
}

type mockFlow struct {
	executeCalled bool
}

func (m *mockFlow) Name() string { return "MockFlow" }
func (m *mockFlow) Execute(ctx context.Context, workspaceService *workspace.Service, thread *chat.Thread, space spaces.ThinkSpace, arguments map[string]any, flowConfiguration flows.FlowConfig, flowContext flows.FlowContext, emitter telemetry.Emitter, executor workspace.SubAgentExecutor, verifier spaces.Verifier) (*flows.FlowResult, error) {
	m.executeCalled = true
	return &flows.FlowResult{
		Branches: []flows.BranchResult{{CandidateID: "candidate/mock-123", Passed: true}},
		Summary:  "Mock delegation complete",
	}, nil
}

type mockUserInterface struct {
	strategyToReturn session.DelegationStrategy
	reviewToReturn   bool
	reviewedBranch   string
}

func (u *mockUserInterface) OnTextChunk(text string)                        {}
func (u *mockUserInterface) ReviewContext(ctxData session.TurnContext) bool { return true }
func (u *mockUserInterface) ChooseNextStep() session.DelegationStrategy     { return u.strategyToReturn }
func (u *mockUserInterface) ReviewCandidate(branch string) bool {
	u.reviewedBranch = branch
	return u.reviewToReturn
}

type emittedEvent struct {
	channel string
	event   telemetry.Event
}

type mockEmitter struct {
	events []emittedEvent
}

func (m *mockEmitter) Emit(channel string, event telemetry.Event) {
	m.events = append(m.events, emittedEvent{channel: channel, event: event})
}

// --- Tests ---

func TestCoordinator_ExecuteTurn_RoutesToDelegator(t *testing.T) {
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
				Candidates: []*genai.Candidate{{
					Content: &genai.Content{
						Parts: []*genai.Part{{
							FunctionCall: &genai.FunctionCall{
								Name: "propose_change",
								Args: map[string]any{},
							},
						}},
					},
				}},
			},
		},
	}

	llmAdapter := llm.NewAdapter(client)
	flow := &mockFlow{}
	executor := func(ctx context.Context, briefing workspace.SubAgentBriefing, sandbox workspace.CandidateSandbox, agentID int, tokenChannel chan<- workspace.AgentToken) error {
		return nil
	}
	emitter := &mockEmitter{}

	delegator := session.NewDelegator(logger, workspaceService, llmAdapter, executor, flow, emitter, "golang", "", flows.FlowConfig{})
	coordinator := session.NewCoordinator(logger, workspaceService, llmAdapter, emitter, delegator)

	userInterface := &mockUserInterface{strategyToReturn: session.StrategyManual, reviewToReturn: true}
	manifest := chat.NewThreadManifest(thread.ID)

	err := coordinator.ExecuteTurn(ctx, thread, manifest, &mockThinkSpace{}, nil, userInterface)
	if err != nil {
		t.Fatalf("ExecuteTurn failed: %v", err)
	}

	if !flow.executeCalled {
		t.Errorf("Expected Flow to be executed upon intercepting tool call via Delegator")
	}
}

func TestCoordinator_QueryLens(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()
	bus := chat.NewEventBus()
	workspaceService := workspace.NewService(logger, &mockChatEngine{}, workspaceRoot, bus)
	thread, _ := workspaceService.StartThread(ctx, "test-thread-lens")

	client := &mockModelClient{
		responses: []*genai.GenerateContentResponse{
			{
				Candidates: []*genai.Candidate{{
					Content: &genai.Content{
						Parts: []*genai.Part{{
							FunctionCall: &genai.FunctionCall{
								Name: "query_lens",
								Args: map[string]any{"tag": "database"},
							},
						}},
					},
				}},
			},
			{
				Candidates: []*genai.Candidate{{
					Content: &genai.Content{
						Parts: []*genai.Part{{Text: "Final answer after querying."}},
					},
				}},
			},
		},
	}

	llmAdapter := llm.NewAdapter(client)
	emitter := &mockEmitter{}
	delegator := session.NewDelegator(logger, workspaceService, llmAdapter, nil, &mockFlow{}, emitter, "golang", "", flows.FlowConfig{})
	coordinator := session.NewCoordinator(logger, workspaceService, llmAdapter, emitter, delegator)

	manifest := chat.NewThreadManifest(thread.ID)
	_ = coordinator.ExecuteTurn(ctx, thread, manifest, &mockThinkSpace{}, nil, &mockUserInterface{})

	foundLensLog := false
	for _, ev := range emitter.events {
		if ev.event.Type == "log" && ev.event.Message == "Manager invoked query_lens for tag: database" {
			foundLensLog = true
		}
	}
	if !foundLensLog {
		t.Errorf("Expected query_lens to trigger a specific manager telemetry log")
	}
}
