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

func (client *csTestModelClient) GenerateContentStream(ctx context.Context, model string, bundle cognition.ThinkBundle, tools []llm.ToolDef) iter.Seq2[llm.StreamChunk, error] {
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
func (space *csTestThinkSpace) Tools() []llm.ToolDef                                   { return nil }
func (space *csTestThinkSpace) Verifier() spaces.Verifier                              { return nil }
func (space *csTestThinkSpace) Mapbook() spaces.Mapbook                                { return nil }
func (space *csTestThinkSpace) Patcher() workspace.Patcher                             { return nil }
func (space *csTestThinkSpace) Initialize(ctx context.Context, directory string) error { return nil }
func (space *csTestThinkSpace) EnrichBundle(ctx context.Context, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus, workspaceRoot string) error {
	return nil
}

type csTestUI struct {
	reviewApproved         bool
	retainedHistoryIndices []int
	retainedFilePaths      []string
	transitionApproved     bool
}

func (ui *csTestUI) OnTextChunk(text string) {}
func (ui *csTestUI) ReviewContext(ctxData session.TurnContext) session.ContextApprovalDecision {
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

func setupSessionTest(t *testing.T, chatID string) (*session.ChatSession, *workspace.ServiceManager, *workspace.Service, *csTestModelClient, string) {
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
	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatEngine { return &csTestChatEngine{} }, graphDB, emitter)
	_ = os.MkdirAll(filepath.Join(workspaceRoot, spaceID), 0755)
	_ = svcManager.UpdateSpaceState(spaceID, func(s *workspace.SpaceState) { s.Domain = "golang" })

	scaffoldTestChat(workspaceRoot, spaceID, chatID)

	registry := config.NewRegistry()
	mockSpace := &csTestThinkSpace{}
	registry.RegisterDomain("golang", mockSpace)
	registry.RegisterConfig("golang", mockSpace.Config())
	registry.RegisterDomain("concept", mockSpace)
	registry.RegisterConfig("concept", mockSpace.Config())

	client := &csTestModelClient{}
	digestManager := session.NewDigestManager(logger, client, session.DigestConfig{})

	chatSession, err := session.OpenChat(context.Background(), logger, registry, svcManager, client, &csTestFlow{}, digestManager, spaceID, chatID)
	if err != nil {
		t.Fatalf("failed to open chat session: %v", err)
	}

	workspaceService := svcManager.GetService(spaceID)

	return chatSession, svcManager, workspaceService, client, spaceID
}

func TestChatSession_RunTurn_Sequencing(t *testing.T) {
	ctx := context.Background()
	chatID := "chat-seq"
	chatSession, svcManager, _, client, spaceID := setupSessionTest(t, chatID)

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
	chatSession, svcManager, _, client, spaceID := setupSessionTest(t, chatID)

	client.responses = []llm.StreamChunk{
		{
			ToolCalls: []llm.ToolCall{
				{
					Name: "transition_workspace",
					Args: map[string]any{"target_domain": "concept", "reasoning": "Need planning"},
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
	if meta.ActiveDomain != "concept" {
		t.Errorf("Expected chat to transition to 'concept', got '%s'", meta.ActiveDomain)
	}
}

func TestChatSession_SpaceContextAssembly(t *testing.T) {
	ctx := context.Background()
	chatID := "chat-context"
	chatSession, svcManager, workspaceService, client, spaceID := setupSessionTest(t, chatID)

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
