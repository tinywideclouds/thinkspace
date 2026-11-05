package session_test

import (
	"context"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"uuid"

	"github.com/dgraph-io/badger/v4"
	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/db"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type csTestModelClient struct {
	responses   []llm.StreamChunk
	callCount   int
	lastRequest cognition.ThinkBundle
}

func (client *csTestModelClient) GenerateContentStream(
	ctx context.Context,
	model string,
	bundle cognition.ThinkBundle,
	registry *tools.Registry,
	config spaces.ThinkStateConfig,
) iter.Seq2[llm.StreamChunk, error] {
	client.lastRequest = bundle
	return func(yield func(llm.StreamChunk, error) bool) {
		if client.callCount < len(client.responses) {
			resp := client.responses[client.callCount]
			client.callCount++
			yield(resp, nil)
		}
	}
}

type csTestChatEngine struct{}

func (engine *csTestChatEngine) InitChat(ctx context.Context, chatID string) error { return nil }
func (engine *csTestChatEngine) Snapshot(ctx context.Context, chatID string, message string) (string, error) {
	return "mock-sha", nil
}
func (engine *csTestChatEngine) SpawnCandidateSandbox(ctx context.Context, chatID, candidateID string) (workspace.CandidateSandbox, error) {
	return nil, nil
}
func (engine *csTestChatEngine) PreviewCandidate(ctx context.Context, chatID, candidateID string) error {
	return nil
}
func (engine *csTestChatEngine) Accept(ctx context.Context, chatID, candidateID, reason string) error {
	return nil
}
func (engine *csTestChatEngine) Reject(ctx context.Context, chatID, candidateID, reason string) error {
	return nil
}
func (engine *csTestChatEngine) ReadCandidateDiff(ctx context.Context, chatID, candidateID string) (string, error) {
	return "", nil
}

type csTestThinkSpace struct{}

func (space *csTestThinkSpace) Config() spaces.ThinkStateConfig {
	cfg := spaces.ThinkStateConfig{
		Roles: spaces.RolesConfig{
			Manager: spaces.ManagerConfig{Model: "test-manager"},
			Worker:  spaces.WorkerConfig{Model: "test-worker"},
		},
		Timeouts: spaces.TimeoutsConfig{TurnSeconds: 300},
	}
	cfg.ApplyDefaults()
	return cfg
}
func (space *csTestThinkSpace) Verifier() spaces.Verifier                              { return nil }
func (space *csTestThinkSpace) Mapbook() spaces.Mapbook                                { return nil }
func (space *csTestThinkSpace) Patcher() workspace.Patcher                             { return nil }
func (space *csTestThinkSpace) Initialize(ctx context.Context, directory string) error { return nil }
func (space *csTestThinkSpace) EnrichBundle(ctx context.Context, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus, workspaceRoot string) error {
	return nil
}

type csTestUI struct {
	reviewContextCalled    bool
	reviewApproved         bool
	retainedHistoryIndices []int
	retainedFilePaths      []string
	transitionApproved     bool
}

func (ui *csTestUI) OnTextChunk(text string) {}
func (ui *csTestUI) ReviewContext(turnContext session.TurnContext) session.ContextApprovalDecision {
	ui.reviewContextCalled = true
	return session.ContextApprovalDecision{
		Approved:               ui.reviewApproved,
		RetainedHistoryIndices: ui.retainedHistoryIndices,
		RetainedFilePaths:      ui.retainedFilePaths,
	}
}
func (ui *csTestUI) ChooseNextStep() flows.FlowStrategy { return flows.StrategyManual }
func (ui *csTestUI) ReviewCandidate(branch string) bool { return true }
func (ui *csTestUI) ConfirmTransition(targetDomain string, reasoning string) bool {
	return ui.transitionApproved
}
func (ui *csTestUI) GetAgentTokenChannel() chan<- workspace.AgentToken { return nil }

type csTestEmitter struct{}

func (emitter *csTestEmitter) Emit(channel string, event telemetry.Event) {}

type csTestFlow struct{}

func (flow *csTestFlow) Name() string  { return "TestFlow" }
func (flow *csTestFlow) MaxSteps() int { return 10 }
func (flow *csTestFlow) Next(state flows.FlowState) flows.FlowDirective {
	return flows.FlowDirective{Action: flows.ActionFinish}
}

func scaffoldTestChat(workspaceRoot, spaceID, chatID string) {
	metaPath := filepath.Join(workspaceRoot, spaceID, "chats", chatID)
	_ = os.MkdirAll(metaPath, 0755)
	_ = os.WriteFile(filepath.Join(metaPath, "meta.json"), []byte(`{"id":"`+chatID+`", "name":"`+chatID+`", "active_branch": "main", "branches": {"main": {"parent_branch": "", "fork_point": ""}}}`), 0644)
}

func setupSessionTest(t *testing.T, chatID string) (*session.Manager, *session.ChatSession, *workspace.ServiceManager, *workspace.Service, *csTestModelClient, string) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()
	spaceID := "test-space"

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("failed to init badger: %v", err)
	}
	t.Cleanup(func() { graphDB.Close() })

	emitter := &csTestEmitter{}
	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatVersioner { return &csTestChatEngine{} }, graphDB, emitter)
	_ = os.MkdirAll(filepath.Join(workspaceRoot, spaceID), 0755)
	_ = svcManager.UpdateSpaceState(spaceID, func(s *workspace.SpaceState) { s.ThinkState = "golang" })

	scaffoldTestChat(workspaceRoot, spaceID, chatID)

	registry := config.NewRegistry()
	mockSpace := &csTestThinkSpace{}
	registry.RegisterThinkState("golang", mockSpace)
	registry.RegisterConfig("golang", mockSpace.Config())
	registry.RegisterThinkState("concept", mockSpace)
	registry.RegisterConfig("concept", mockSpace.Config())

	intakeCfg := spaces.ThinkStateConfig{
		Name: "intake",
		Roles: spaces.RolesConfig{
			Manager: spaces.ManagerConfig{
				Model:        "test-manager",
				SystemPrompt: "Routing targets:\n\n{{.AvailableThinkStates}}",
			},
		},
	}
	intakeCfg.ApplyDefaults()
	registry.RegisterThinkState("intake", &csTestThinkSpace{})
	registry.RegisterConfig("intake", intakeCfg)

	client := &csTestModelClient{}
	digestManager := session.NewDigestManager(logger, client, session.DigestConfig{})

	turnManager := session.NewManager(logger, registry, svcManager, client, &csTestFlow{}, digestManager)

	chatSession, err := turnManager.GetSession(context.Background(), spaceID, chatID)
	if err != nil {
		t.Fatalf("failed to open chat session: %v", err)
	}

	workspaceService := svcManager.GetService(spaceID)

	return turnManager, chatSession, svcManager, workspaceService, client, spaceID
}

func TestChatSession_RunTurn_Sequencing(t *testing.T) {
	ctx := context.Background()
	chatID := "chat-seq"
	_, chatSession, svcManager, _, client, spaceID := setupSessionTest(t, chatID)

	_ = svcManager.UpdateSpaceSettings(spaceID, func(s *workspace.SpaceSettings) {
		s.ReviewPolicy.Active = true
		s.ReviewPolicy.MinTokens = 1
	})

	client.responses = []llm.StreamChunk{{Text: "Test standard response"}}
	ui := &csTestUI{reviewApproved: true}
	emitter := &csTestEmitter{}

	err := chatSession.RunTurn(ctx, "Hello agent", ui, emitter)
	if err != nil {
		t.Fatalf("RunTurn failed: %v", err)
	}

	chat := &thinkgraph.Chat{ID: chatID, SpaceID: spaceID, ActiveBranch: "main"}
	timeline, err := svcManager.GraphRepository().GetBranchTimeline(ctx, chat, "main")
	if err != nil {
		t.Fatalf("Failed to fetch timeline: %v", err)
	}

	if len(timeline) == 0 {
		t.Errorf("Expected timeline to contain events, got 0")
	}
}

func TestChatSession_RunTurn_Transition(t *testing.T) {
	ctx := context.Background()
	chatID := "chat-trans"
	_, chatSession, svcManager, _, client, spaceID := setupSessionTest(t, chatID)

	_ = svcManager.UpdateSpaceSettings(spaceID, func(s *workspace.SpaceSettings) {
		s.ReviewPolicy.Active = true
		s.ReviewPolicy.MinTokens = 1
	})

	client.responses = []llm.StreamChunk{
		{
			ToolCalls: []llm.ToolCall{
				{
					Name: "transition_workspace",
					Args: map[string]any{"target_think_state": "concept", "reasoning": "Need planning"},
				},
			},
		},
		{
			Text: "Now executing in concept domain.",
		},
	}

	ui := &csTestUI{reviewApproved: true, transitionApproved: true}
	emitter := &csTestEmitter{}

	err := chatSession.RunTurn(ctx, "Let's plan", ui, emitter)
	if err != nil {
		t.Fatalf("RunTurn failed: %v", err)
	}

	meta, err := svcManager.GetChatMeta(spaceID, chatID)
	if err != nil {
		t.Fatalf("Failed to fetch chat meta: %v", err)
	}
	if meta.ActiveThinkState != "concept" {
		t.Errorf("Expected chat to transition to 'concept', got '%s'", meta.ActiveThinkState)
	}
}

func TestChatSession_SpaceContextAssembly(t *testing.T) {
	ctx := context.Background()
	chatID := "chat-context"
	_, chatSession, svcManager, workspaceService, client, spaceID := setupSessionTest(t, chatID)

	_ = svcManager.UpdateSpaceSettings(spaceID, func(s *workspace.SpaceSettings) {
		s.ReviewPolicy.Active = true
		s.ReviewPolicy.MinTokens = 1
	})

	chat := &thinkgraph.Chat{ID: chatID, SpaceID: spaceID, ActiveBranch: "main"}

	id := uuid.NewV7()
	_ = svcManager.GraphRepository().SaveSpaceContext(ctx, chat, thinkgraph.SpaceContext{
		ID:      id,
		Content: "Always use PostgreSQL.",
		Tags:    []string{"database"},
	})

	ev := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Build database")
	ev.Tags = []string{"database"}
	ev.BranchID = "main"

	_ = workspaceService.Dispatcher().AppendEvent(ctx, chat, ev)

	client.responses = []llm.StreamChunk{{Text: "Ok"}}
	ui := &csTestUI{reviewApproved: true}
	emitter := &csTestEmitter{}

	_ = chatSession.RunTurn(ctx, "Start", ui, emitter)

	foundRule := false
	for _, memory := range client.lastRequest.Trajectory.LongTermMemory {
		if strings.Contains(memory, "PostgreSQL") {
			foundRule = true
			break
		}
	}

	if !foundRule {
		t.Errorf("ChatSession failed to assemble targeted SpaceContext rules into LLM context bundle")
	}
}

func TestChatSession_RunTurn_BypassesReviewOnLowTokenThreshold(t *testing.T) {
	ctx := context.Background()
	chatID := "chat-bypass-threshold"
	_, chatSession, svcManager, _, client, spaceID := setupSessionTest(t, chatID)

	_ = svcManager.UpdateSpaceSettings(spaceID, func(s *workspace.SpaceSettings) {
		s.ReviewPolicy.Active = true
		s.ReviewPolicy.MinTokens = 5000
	})

	client.responses = []llm.StreamChunk{{Text: "Hello there!"}}
	ui := &csTestUI{reviewApproved: false}
	emitter := &csTestEmitter{}

	err := chatSession.RunTurn(ctx, "hi", ui, emitter)
	if err != nil {
		t.Fatalf("RunTurn failed: %v", err)
	}

	if ui.reviewContextCalled {
		t.Errorf("expected ui.ReviewContext to be bypassed when below token threshold, but it was called")
	}
}

func TestChatSession_RunTurn_BypassesReviewWhenDisabledGlobally(t *testing.T) {
	ctx := context.Background()
	chatID := "chat-bypass-disabled"
	_, chatSession, svcManager, _, client, spaceID := setupSessionTest(t, chatID)

	_ = svcManager.UpdateSpaceSettings(spaceID, func(s *workspace.SpaceSettings) {
		s.ReviewPolicy.Active = false
		s.ReviewPolicy.MinTokens = 1
	})

	client.responses = []llm.StreamChunk{{Text: "Direct generation"}}
	ui := &csTestUI{reviewApproved: false}
	emitter := &csTestEmitter{}

	err := chatSession.RunTurn(ctx, "process everything", ui, emitter)
	if err != nil {
		t.Fatalf("RunTurn failed: %v", err)
	}

	if ui.reviewContextCalled {
		t.Errorf("expected ui.ReviewContext to be bypassed when ContextReviewEnabled is false, but it was called")
	}
}

func TestChatSession_IntakeBootAndContextInjection(t *testing.T) {
	ctx := context.Background()
	chatID := "chat-intake"

	turnManager, _, svcManager, _, client, spaceID := setupSessionTest(t, chatID)

	_ = svcManager.UpdateChatMeta(spaceID, chatID, func(m *workspace.ChatMeta) {
		m.ActiveThinkState = ""
	})
	_ = svcManager.UpdateSpaceState(spaceID, func(s *workspace.SpaceState) {
		s.ThinkState = ""
	})

	chatSession, _ := turnManager.GetSession(ctx, spaceID, chatID)

	client.responses = []llm.StreamChunk{{Text: "Hello there."}}
	ui := &csTestUI{reviewApproved: true}
	emitter := &csTestEmitter{}

	err := chatSession.RunTurn(ctx, "hi", ui, emitter)
	if err != nil {
		t.Fatalf("RunTurn failed: %v", err)
	}

	meta, _ := svcManager.GetChatMeta(spaceID, chatID)
	if meta.ActiveThinkState != "intake" {
		t.Errorf("Expected ActiveThinkState to default to 'intake', got '%s'", meta.ActiveThinkState)
	}

	persona := client.lastRequest.Rules.Persona
	if strings.Contains(persona, "{{.AvailableThinkStates}}") {
		t.Errorf("Expected placeholder to be replaced, but it was found in the persona output")
	}
}

func TestChatSession_ContextReview_Retentions(t *testing.T) {
	ctx := context.Background()
	chatID := "chat-retention"
	_, chatSession, svcManager, workspaceService, client, spaceID := setupSessionTest(t, chatID)

	_ = svcManager.UpdateSpaceSettings(spaceID, func(s *workspace.SpaceSettings) {
		s.ReviewPolicy.Active = true
		s.ReviewPolicy.MinTokens = 1
	})

	chat := &thinkgraph.Chat{ID: chatID, SpaceID: spaceID, ActiveBranch: "main"}
	ev := thinkgraph.NewEvent(thinkgraph.EventModel, "First response")
	_ = workspaceService.Dispatcher().AppendEvent(ctx, chat, ev)

	client.responses = []llm.StreamChunk{{Text: "Ok"}}

	ui := &csTestUI{
		reviewApproved:         true,
		retainedHistoryIndices: []int{1},
	}
	emitter := &csTestEmitter{}

	_ = chatSession.RunTurn(ctx, "New task", ui, emitter)

	if len(client.lastRequest.Trajectory.WorkingMemory) != 0 {
		t.Errorf("Expected working memory to be fully pruned by UI retention, got %d items", len(client.lastRequest.Trajectory.WorkingMemory))
	}
	if client.lastRequest.Trajectory.Catalyst.Content != "New task" {
		t.Errorf("Expected catalyst (index 1) to survive pruning, got '%s'", client.lastRequest.Trajectory.Catalyst.Content)
	}
}

func TestChatSession_ContextReview_AbortOnMissingCatalyst(t *testing.T) {
	ctx := context.Background()
	chatID := "chat-abort"
	_, chatSession, svcManager, _, client, spaceID := setupSessionTest(t, chatID)

	_ = svcManager.UpdateSpaceSettings(spaceID, func(s *workspace.SpaceSettings) {
		s.ReviewPolicy.Active = true
		s.ReviewPolicy.MinTokens = 1
	})

	client.responses = []llm.StreamChunk{{Text: "This should not be called"}}

	ui := &csTestUI{
		reviewApproved:         true,
		retainedHistoryIndices: []int{},
	}
	emitter := &csTestEmitter{}

	err := chatSession.RunTurn(ctx, "Ignored task", ui, emitter)
	if err != nil {
		t.Fatalf("RunTurn failed unexpectedly: %v", err)
	}

	if client.callCount > 0 {
		t.Errorf("Expected LLM generation to be completely aborted if the Catalyst is unchecked")
	}
}
