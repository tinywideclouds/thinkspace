package session_test

import (
	"context"
	"fmt"
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
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type ceTestModelClient struct {
	responses   []llm.StreamChunk
	callCount   int
	lastRequest cognition.ThinkBundle
}

func (client *ceTestModelClient) GenerateContentStream(
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

type ceTestChatEngine struct{}

func (engine *ceTestChatEngine) InitChat(ctx context.Context, chatID string) error { return nil }
func (engine *ceTestChatEngine) Snapshot(ctx context.Context, chatID string, message string) (string, error) {
	return "mock-sha", nil
}
func (engine *ceTestChatEngine) SpawnCandidateSandbox(ctx context.Context, chatID, candidateID string) (workspace.CandidateSandbox, error) {
	return nil, nil
}
func (engine *ceTestChatEngine) PreviewCandidate(ctx context.Context, chatID, candidateID string) error {
	return nil
}
func (engine *ceTestChatEngine) Accept(ctx context.Context, chatID, candidateID, reason string) error {
	return nil
}
func (engine *ceTestChatEngine) Reject(ctx context.Context, chatID, candidateID, reason string) error {
	return nil
}
func (engine *ceTestChatEngine) ReadCandidateDiff(ctx context.Context, chatID, candidateID string) (string, error) {
	return "", nil
}

type ceTestThinkSpace struct{}

func (space *ceTestThinkSpace) Config() spaces.ThinkStateConfig {
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
func (space *ceTestThinkSpace) Verifier() spaces.Verifier { return nil }
func (space *ceTestThinkSpace) Mapbook() spaces.Mapbook   { return space }
func (space *ceTestThinkSpace) GenerateLayers(ctx context.Context, workspaceRoot string) (map[spaces.MapbookLayer]string, error) {
	return map[spaces.MapbookLayer]string{}, nil
}
func (space *ceTestThinkSpace) Patcher() workspace.Patcher                             { return nil }
func (space *ceTestThinkSpace) Initialize(ctx context.Context, directory string) error { return nil }
func (space *ceTestThinkSpace) EnrichBundle(ctx context.Context, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus, workspaceRoot string) error {
	return nil
}

type ceTestUI struct {
	reviewContextCalled    bool
	reviewApproved         bool
	retainedHistoryIndices []int
	retainedFilePaths      []string
	transitionApproved     bool
}

func (ui *ceTestUI) OnTextChunk(text string) {}
func (ui *ceTestUI) ReviewContext(turnContext session.TurnContext) session.ContextApprovalDecision {
	ui.reviewContextCalled = true
	return session.ContextApprovalDecision{
		Approved:               ui.reviewApproved,
		RetainedHistoryIndices: ui.retainedHistoryIndices,
		RetainedFilePaths:      ui.retainedFilePaths,
	}
}
func (ui *ceTestUI) ChooseNextStep() flows.FlowStrategy { return flows.StrategyManual }
func (ui *ceTestUI) ReviewCandidate(branch string) bool { return true }
func (ui *ceTestUI) ConfirmTransition(req signals.SignalTransition) bool {
	return ui.transitionApproved
}
func (ui *ceTestUI) GetAgentTokenChannel() chan<- workspace.AgentToken { return nil }

type ceTestEmitter struct{}

func (emitter *ceTestEmitter) Emit(channel string, event telemetry.Event) {}

type ceTestFlow struct{}

func (flow *ceTestFlow) Name() string  { return "TestFlow" }
func (flow *ceTestFlow) MaxSteps() int { return 10 }
func (flow *ceTestFlow) Next(state flows.FlowState) flows.FlowDirective {
	return flows.FlowDirective{Action: flows.ActionFinish}
}

func scaffoldChat(workspaceRoot, spaceID, chatID string, graphID thinkgraph.GraphID) {
	metaPath := filepath.Join(workspaceRoot, spaceID, "chats", chatID)
	_ = os.MkdirAll(metaPath, 0755)
	
	metaJSON := fmt.Sprintf(`{"id":"%s", "name":"%s", "active_branch": "main", "branches": {"main": {"parent_branch": "", "fork_point": "", "graph_id": "%s"}}}`, chatID, chatID, graphID.String())
	_ = os.WriteFile(filepath.Join(metaPath, "meta.json"), []byte(metaJSON), 0644)
}

func setupEngineTest(t *testing.T, chatID string) (*session.Manager, *session.ChatSession, *workspace.ServiceManager, *workspace.Service, *ceTestModelClient, string, *config.Registry, thinkgraph.GraphID) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()
	spaceID := "test-space"
	graphID := thinkgraph.NewGraphID(thinkgraph.PrefixChat)

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("failed to init badger: %v", err)
	}
	t.Cleanup(func() { graphDB.Close() })

	emitter := &ceTestEmitter{}
	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatVersioner { return &ceTestChatEngine{} }, graphDB, emitter)
	_ = os.MkdirAll(filepath.Join(workspaceRoot, spaceID), 0755)
	_ = svcManager.UpdateSpaceState(spaceID, func(s *workspace.SpaceState) { s.ThinkState = "golang" })

	scaffoldChat(workspaceRoot, spaceID, chatID, graphID)

	registry := config.NewRegistry()
	mockSpace := &ceTestThinkSpace{}
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
	registry.RegisterThinkState("intake", &ceTestThinkSpace{})
	registry.RegisterConfig("intake", intakeCfg)

	client := &ceTestModelClient{}
	digestManager := session.NewDigestManager(logger, client, session.DigestConfig{})

	turnManager := session.NewManager(logger, registry, svcManager, client, &ceTestFlow{}, digestManager)

	chatSession, err := turnManager.GetSession(context.Background(), spaceID, chatID)
	if err != nil {
		t.Fatalf("failed to open chat session: %v", err)
	}

	workspaceService := svcManager.GetService(spaceID)

	return turnManager, chatSession, svcManager, workspaceService, client, spaceID, registry, graphID
}

func TestChatEngine_Run_Sequencing(t *testing.T) {
	ctx := context.Background()
	chatID := "chat-seq"
	_, chatSession, svcManager, _, client, spaceID, _, graphID := setupEngineTest(t, chatID)

	_ = svcManager.UpdateSpaceSettings(spaceID, func(s *workspace.SpaceSettings) {
		s.ReviewPolicy.Active = true
		s.ReviewPolicy.MinTokens = 1
	})

	client.responses = []llm.StreamChunk{{Text: "Test standard response"}}
	ui := &ceTestUI{reviewApproved: true}
	emitter := &ceTestEmitter{}

	err := chatSession.RunTurn(ctx, "Hello agent", ui, emitter)
	if err != nil {
		t.Fatalf("RunTurn failed: %v", err)
	}

	address := &thinkgraph.ChatAddress{
		ChatID: chatID, ProjectID: spaceID,
		SubGraph: thinkgraph.SubGraph{GraphID: graphID, Name: "main"},
	}

	timeline, err := svcManager.GraphRepository().GetSubGraphTimeline(ctx, address, address.GraphID)
	if err != nil {
		t.Fatalf("Failed to fetch timeline: %v", err)
	}

	if len(timeline) == 0 {
		t.Errorf("Expected timeline to contain events, got 0")
	}
}

func TestChatEngine_Run_Transition(t *testing.T) {
	ctx := context.Background()
	chatID := "chat-trans"
	_, chatSession, svcManager, _, client, spaceID, _, _ := setupEngineTest(t, chatID)

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

	ui := &ceTestUI{reviewApproved: true, transitionApproved: true}
	emitter := &ceTestEmitter{}

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

func TestChatEngine_SpaceContextAssembly(t *testing.T) {
	ctx := context.Background()
	chatID := "chat-context"
	_, chatSession, svcManager, workspaceService, client, spaceID, _, graphID := setupEngineTest(t, chatID)

	_ = svcManager.UpdateSpaceSettings(spaceID, func(s *workspace.SpaceSettings) {
		s.ReviewPolicy.Active = true
		s.ReviewPolicy.MinTokens = 1
	})

	address := &thinkgraph.ChatAddress{
		ChatID: chatID, ProjectID: spaceID,
		SubGraph: thinkgraph.SubGraph{GraphID: graphID, Name: "main"},
	}

	id := uuid.NewV7()
	_ = svcManager.GraphRepository().SaveSpaceContext(ctx, address, thinkgraph.SpaceContext{
		ID:      id,
		Content: "Always use PostgreSQL.",
		Tags:    []string{"database"},
	})

	ev := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Build database")
	ev.Tags = []string{"database"}

	_ = workspaceService.Dispatcher().AppendEvent(ctx, address, ev)

	client.responses = []llm.StreamChunk{{Text: "Ok"}}
	ui := &ceTestUI{reviewApproved: true}
	emitter := &ceTestEmitter{}

	_ = chatSession.RunTurn(ctx, "Start", ui, emitter)

	foundRule := false
	for _, memory := range client.lastRequest.Trajectory.LongTermMemory {
		if strings.Contains(memory, "PostgreSQL") {
			foundRule = true
			break
		}
	}

	if !foundRule {
		t.Errorf("ChatEngine failed to assemble targeted SpaceContext rules into LLM context bundle")
	}
}

func TestChatEngine_Run_BypassesReviewOnLowTokenThreshold(t *testing.T) {
	ctx := context.Background()
	chatID := "chat-bypass-threshold"
	_, chatSession, svcManager, _, client, spaceID, _, _ := setupEngineTest(t, chatID)

	_ = svcManager.UpdateSpaceSettings(spaceID, func(s *workspace.SpaceSettings) {
		s.ReviewPolicy.Active = true
		s.ReviewPolicy.MinTokens = 5000
	})

	client.responses = []llm.StreamChunk{{Text: "Hello there!"}}
	ui := &ceTestUI{reviewApproved: false}
	emitter := &ceTestEmitter{}

	err := chatSession.RunTurn(ctx, "hi", ui, emitter)
	if err != nil {
		t.Fatalf("RunTurn failed: %v", err)
	}

	if ui.reviewContextCalled {
		t.Errorf("expected ui.ReviewContext to be bypassed when below token threshold, but it was called")
	}
}

func TestChatEngine_Run_BypassesReviewWhenDisabledGlobally(t *testing.T) {
	ctx := context.Background()
	chatID := "chat-bypass-disabled"
	_, chatSession, svcManager, _, client, spaceID, _, _ := setupEngineTest(t, chatID)

	_ = svcManager.UpdateSpaceSettings(spaceID, func(s *workspace.SpaceSettings) {
		s.ReviewPolicy.Active = false
		s.ReviewPolicy.MinTokens = 1
	})

	client.responses = []llm.StreamChunk{{Text: "Direct generation"}}
	ui := &ceTestUI{reviewApproved: false}
	emitter := &ceTestEmitter{}

	err := chatSession.RunTurn(ctx, "process everything", ui, emitter)
	if err != nil {
		t.Fatalf("RunTurn failed: %v", err)
	}

	if ui.reviewContextCalled {
		t.Errorf("expected ui.ReviewContext to be bypassed when ContextReviewEnabled is false, but it was called")
	}
}

func TestChatEngine_IntakeBootAndContextInjection(t *testing.T) {
	ctx := context.Background()
	chatID := "chat-intake"

	_, _, serviceManager, _, client, spaceID, registry, graphID := setupEngineTest(t, chatID)

	_ = serviceManager.UpdateChatMeta(spaceID, chatID, func(m *workspace.ChatMeta) {
		m.ActiveThinkState = ""
	})
	_ = serviceManager.UpdateSpaceState(spaceID, func(s *workspace.SpaceState) {
		s.ThinkState = ""
	})

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	digestManager := session.NewDigestManager(logger, client, session.DigestConfig{})

	chatSession, _ := session.NewChatSession(ctx, logger, registry, serviceManager, client, &ceTestFlow{}, digestManager, spaceID, chatID)

	chatSession.Mu().Lock()
	meta := chatSession.Meta
	branch := meta.Branches["main"]
	branch.GraphID = graphID.String()
	meta.Branches["main"] = branch
	chatSession.Meta = meta
	chatSession.Mu().Unlock()

	client.responses = []llm.StreamChunk{{Text: "Hello there."}}
	ui := &ceTestUI{reviewApproved: true}
	emitter := &ceTestEmitter{}

	err := chatSession.RunTurn(ctx, "hi", ui, emitter)
	if err != nil {
		t.Fatalf("RunTurn failed: %v", err)
	}

	meta, _ = serviceManager.GetChatMeta(spaceID, chatID)
	if meta.ActiveThinkState != "intake" {
		t.Errorf("Expected ActiveThinkState to default to 'intake', got '%s'", meta.ActiveThinkState)
	}

	persona := client.lastRequest.Rules.Persona
	if strings.Contains(persona, "{{.AvailableThinkStates}}") {
		t.Errorf("Expected placeholder to be replaced, but it was found in the persona output")
	}
}

func TestChatEngine_ContextReview_Retentions(t *testing.T) {
	ctx := context.Background()
	chatID := "chat-retention"
	_, chatSession, svcManager, workspaceService, client, spaceID, _, graphID := setupEngineTest(t, chatID)

	_ = svcManager.UpdateSpaceSettings(spaceID, func(s *workspace.SpaceSettings) {
		s.ReviewPolicy.Active = true
		s.ReviewPolicy.MinTokens = 1
	})

	address := &thinkgraph.ChatAddress{
		ChatID: chatID, ProjectID: spaceID,
		SubGraph: thinkgraph.SubGraph{GraphID: graphID, Name: "main"},
	}
	
	// Create multiple history events so the retention indices filter them
	ev1 := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Initial context")
	_ = workspaceService.Dispatcher().AppendEvent(ctx, address, ev1)

	ev2 := thinkgraph.NewEvent(thinkgraph.EventModel, "First response")
	_ = workspaceService.Dispatcher().AppendEvent(ctx, address, ev2)

	client.responses = []llm.StreamChunk{{Text: "Ok"}}

	ui := &ceTestUI{
		reviewApproved:         true,
		retainedHistoryIndices: []int{0, 2}, // Keep 'Initial Context' (0) and Catalyst (2)
	}
	emitter := &ceTestEmitter{}

	_ = chatSession.RunTurn(ctx, "New task", ui, emitter)

	if len(client.lastRequest.Trajectory.WorkingMemory) != 1 {
		t.Errorf("Expected working memory to be fully pruned down to retained index, got %d items", len(client.lastRequest.Trajectory.WorkingMemory))
	} else if client.lastRequest.Trajectory.WorkingMemory[0].Content != "Initial context" {
		t.Errorf("Expected retained context to be 'Initial context', got '%s'", client.lastRequest.Trajectory.WorkingMemory[0].Content)
	}

	if client.lastRequest.Trajectory.Catalyst.Content != "New task" {
		t.Errorf("Expected catalyst to survive pruning, got '%s'", client.lastRequest.Trajectory.Catalyst.Content)
	}
}

func TestChatEngine_ContextReview_AbortOnMissingCatalyst(t *testing.T) {
	ctx := context.Background()
	chatID := "chat-abort"
	_, chatSession, svcManager, _, client, spaceID, _, _ := setupEngineTest(t, chatID)

	_ = svcManager.UpdateSpaceSettings(spaceID, func(s *workspace.SpaceSettings) {
		s.ReviewPolicy.Active = true
		s.ReviewPolicy.MinTokens = 1
	})

	client.responses = []llm.StreamChunk{{Text: "This should not be called"}}

	ui := &ceTestUI{
		reviewApproved:         true,
		retainedHistoryIndices: []int{}, // Empty array discards the catalyst
	}
	emitter := &ceTestEmitter{}

	err := chatSession.RunTurn(ctx, "Ignored task", ui, emitter)
	if err != nil {
		t.Fatalf("RunTurn failed unexpectedly: %v", err)
	}

	if client.callCount > 0 {
		t.Errorf("Expected LLM generation to be completely aborted if the Catalyst is unchecked")
	}
}