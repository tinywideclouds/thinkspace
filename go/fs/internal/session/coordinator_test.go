package session_test

import (
	"context"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

type coordMockModelClient struct {
	responses      []llm.StreamChunk
	callIndex      int
	capturedBundle cognition.ThinkBundle
}

func (m *coordMockModelClient) GenerateContentStream(ctx context.Context, model string, bundle cognition.ThinkBundle, tools []llm.ToolDef) iter.Seq2[llm.StreamChunk, error] {
	m.capturedBundle = bundle
	return func(yield func(llm.StreamChunk, error) bool) {
		if m.callIndex < len(m.responses) {
			resp := m.responses[m.callIndex]
			m.callIndex++
			yield(resp, nil)
		}
	}
}

type coordMockChatEngine struct{}

func (m *coordMockChatEngine) InitChat(ctx context.Context, chatID string) error { return nil }
func (m *coordMockChatEngine) Snapshot(ctx context.Context, chatID string, message string) (string, error) {
	return "mock-sha", nil
}
func (m *coordMockChatEngine) SpawnCandidateSandbox(ctx context.Context, chatID string, candidateID string) (workspace.CandidateSandbox, error) {
	return nil, nil
}
func (m *coordMockChatEngine) PreviewCandidate(ctx context.Context, chatID string, candidateID string) error {
	return nil
}
func (m *coordMockChatEngine) Accept(ctx context.Context, chatID string, candidateID string, reason string) error {
	return nil
}
func (m *coordMockChatEngine) Reject(ctx context.Context, chatID string, candidateID string, reason string) error {
	return nil
}
func (m *coordMockChatEngine) ReadCandidateDiff(ctx context.Context, chatID, candidateID string) (string, error) {
	return "+ mock diff", nil
}

type coordMockThinkSpace struct{}

func (m *coordMockThinkSpace) Config() spaces.ThinkStateConfig {
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
func (m *coordMockThinkSpace) Tools() []llm.ToolDef                                   { return nil }
func (m *coordMockThinkSpace) Verifier() spaces.Verifier                              { return nil }
func (m *coordMockThinkSpace) Mapbook() spaces.Mapbook                                { return nil }
func (m *coordMockThinkSpace) Patcher() workspace.Patcher                             { return nil }
func (m *coordMockThinkSpace) Initialize(ctx context.Context, directory string) error { return nil }
func (m *coordMockThinkSpace) EnrichBundle(ctx context.Context, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus, workspaceRoot string) error {
	return nil
}

type coordMockFlow struct {
	executeCalled bool
}

func (m *coordMockFlow) Name() string  { return "MockFlow" }
func (m *coordMockFlow) MaxSteps() int { return 10 }
func (m *coordMockFlow) Next(state flows.FlowState) flows.FlowDirective {
	m.executeCalled = true
	return flows.FlowDirective{Action: flows.ActionFinish}
}

type coordMockUI struct {
	strategyToReturn   flows.FlowStrategy
	reviewToReturn     bool
	reviewedBranch     string
	transitionToReturn bool
	capturedReasoning  string
	retainedFiles      []string
	retainedHistory    []int
}

func (u *coordMockUI) OnTextChunk(text string) {}
func (u *coordMockUI) ReviewContext(ctxData session.TurnContext) session.ContextApprovalDecision {
	return session.ContextApprovalDecision{
		Approved:               u.reviewToReturn,
		RetainedFilePaths:      u.retainedFiles,
		RetainedHistoryIndices: u.retainedHistory,
	}
}
func (u *coordMockUI) ChooseNextStep() flows.FlowStrategy { return u.strategyToReturn }
func (u *coordMockUI) ReviewCandidate(branch string) bool {
	u.reviewedBranch = branch
	return u.reviewToReturn
}
func (u *coordMockUI) ConfirmTransition(targetDomain string, reasoning string) bool {
	u.capturedReasoning = reasoning
	return u.transitionToReturn
}

type coordEmittedEvent struct {
	channel string
	event   telemetry.Event
}
type coordMockEmitter struct {
	events []coordEmittedEvent
}

func (m *coordMockEmitter) Emit(channel string, event telemetry.Event) {
	m.events = append(m.events, coordEmittedEvent{channel: channel, event: event})
}

func TestCoordinator_RunAgentLoop_RoutesToEngine(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()
	bus := thinkgraph.NewEventBus()
	bus.Subscribe(thinkgraph.NewLedgerSubscriber(1000, workspaceRoot))

	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatEngine { return &coordMockChatEngine{} }, nil)
	workspaceService := workspace.NewService(logger, &coordMockChatEngine{}, workspaceRoot, bus, nil)
	chatObj, _ := workspaceService.InitializeChat(ctx, "golang", "test-thread")

	scaffoldTestChat(workspaceRoot, "golang", "test-thread")
	activeSession, _ := session.OpenChat(ctx, logger, config.NewRegistry(), svcManager, nil, nil, nil, "golang", "test-thread")

	client := &coordMockModelClient{
		responses: []llm.StreamChunk{
			{
				ToolCalls: []llm.ToolCall{{
					Name: "propose_change",
					Args: map[string]any{},
				}},
			},
		},
	}

	flow := &coordMockFlow{}
	digestManager := session.NewDigestManager(logger, client, session.DigestConfig{})

	emitter := &coordMockEmitter{}
	flowEngine := session.NewFlowEngine(logger, workspaceService, nil, emitter)
	coordinator := session.NewCoordinator(logger, workspaceService, client, emitter, flowEngine, flow, digestManager, "golang", "")

	ui := &coordMockUI{strategyToReturn: flows.StrategyManual, reviewToReturn: true}

	err := coordinator.RunAgentLoop(ctx, chatObj, activeSession, &coordMockThinkSpace{}, cognition.ThinkBundle{}, ui, nil)
	if err != nil {
		t.Fatalf("RunAgentLoop failed: %v", err)
	}

	if !flow.executeCalled {
		t.Errorf("Expected Flow to be evaluated by the Engine upon intercepting tool call")
	}
}

func TestCoordinator_QueryLens(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()
	bus := thinkgraph.NewEventBus()

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("failed to init badger: %v", err)
	}
	defer graphDB.Close()

	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatEngine { return &coordMockChatEngine{} }, graphDB)
	workspaceService := workspace.NewService(logger, &coordMockChatEngine{}, workspaceRoot, bus, graphDB)
	chatObj, _ := workspaceService.InitializeChat(ctx, "golang", "test-thread-lens")

	scaffoldTestChat(workspaceRoot, "golang", "test-thread-lens")
	activeSession, _ := session.OpenChat(ctx, logger, config.NewRegistry(), svcManager, nil, nil, nil, "golang", "test-thread-lens")

	ev := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Historical DB context!")
	ev.Tags = []string{"database"}
	_ = graphDB.AppendEvent(ctx, chatObj, ev)

	client := &coordMockModelClient{
		responses: []llm.StreamChunk{
			{
				ToolCalls: []llm.ToolCall{{
					Name: "query_lens",
					Args: map[string]any{"tag": "database"},
				}},
			},
			{
				Text: "Final answer after querying.",
			},
		},
	}

	emitter := &coordMockEmitter{}
	flowEngine := session.NewFlowEngine(logger, workspaceService, nil, emitter)
	digestManager := session.NewDigestManager(logger, client, session.DigestConfig{})
	coordinator := session.NewCoordinator(logger, workspaceService, client, emitter, flowEngine, &coordMockFlow{}, digestManager, "golang", "")

	_ = coordinator.RunAgentLoop(ctx, chatObj, activeSession, &coordMockThinkSpace{}, cognition.ThinkBundle{}, &coordMockUI{reviewToReturn: true}, nil)

	foundLensLog := false
	for _, e := range emitter.events {
		if e.event.Type == "log" && e.event.Message == "Manager invoked query_lens for tag: database" {
			foundLensLog = true
		}
	}
	if !foundLensLog {
		t.Errorf("Expected query_lens to trigger a specific manager telemetry log")
	}
}

func TestCoordinator_CreateDigest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()
	bus := thinkgraph.NewEventBus()
	bus.Subscribe(thinkgraph.NewLedgerSubscriber(1000, workspaceRoot))
	bus.Subscribe(thinkgraph.NewDigestSubscriber(workspaceRoot))

	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatEngine { return &coordMockChatEngine{} }, nil)
	workspaceService := workspace.NewService(logger, &coordMockChatEngine{}, workspaceRoot, bus, nil)
	chatObj, _ := workspaceService.InitializeChat(ctx, "golang", "test-thread-digest")

	scaffoldTestChat(workspaceRoot, "golang", "test-thread-digest")
	activeSession, _ := session.OpenChat(ctx, logger, config.NewRegistry(), svcManager, nil, nil, nil, "golang", "test-thread-digest")

	client := &coordMockModelClient{
		responses: []llm.StreamChunk{
			{
				ToolCalls: []llm.ToolCall{{
					Name: "create_digest",
					Args: map[string]any{
						"topic_name":  "Memory Compression",
						"tags":        []any{"auth"},
						"instruction": "Summarize the auth discussion.",
					},
				}},
			},
			{
				Text: "Generated Manual Summary",
			},
		},
	}

	emitter := &coordMockEmitter{}
	digestManager := session.NewDigestManager(logger, client, session.DigestConfig{
		ManualPrompt: "Instruction: {{.Instruction}}\nContext: {{.Context}}",
	})
	flowEngine := session.NewFlowEngine(logger, workspaceService, nil, emitter)
	coordinator := session.NewCoordinator(logger, workspaceService, client, emitter, flowEngine, &coordMockFlow{}, digestManager, "golang", "")

	_ = coordinator.RunAgentLoop(ctx, chatObj, activeSession, &coordMockThinkSpace{}, cognition.ThinkBundle{}, &coordMockUI{reviewToReturn: true}, nil)

	digestsDir := filepath.Join(workspaceRoot, "golang", "chats", chatObj.ID, "ledger", "digests")
	entries, err := os.ReadDir(digestsDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("Expected create_digest tool to route execution to the DigestManager and write to ledger digests dir")
	}

	content, err := os.ReadFile(filepath.Join(digestsDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("Failed to read digest file")
	}

	output := string(content)
	if !strings.Contains(output, "Generated Manual Summary") {
		t.Errorf("Expected digest body in markdown, got: %s", output)
	}
	if !strings.Contains(output, "auth") {
		t.Errorf("Expected 'auth' tag in markdown frontmatter, got: %s", output)
	}
}

func TestCoordinator_TransitionWorkspace_Approved(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()
	bus := thinkgraph.NewEventBus()

	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatEngine { return &coordMockChatEngine{} }, nil)
	workspaceService := workspace.NewService(logger, &coordMockChatEngine{}, workspaceRoot, bus, nil)
	chatObj, _ := workspaceService.InitializeChat(ctx, "concept", "test-thread-trans")

	scaffoldTestChat(workspaceRoot, "concept", "test-thread-trans")
	activeSession, _ := session.OpenChat(ctx, logger, config.NewRegistry(), svcManager, nil, nil, nil, "concept", "test-thread-trans")

	client := &coordMockModelClient{
		responses: []llm.StreamChunk{
			{
				ToolCalls: []llm.ToolCall{{
					Name: "transition_workspace",
					Args: map[string]any{
						"target_domain": "golang-space",
						"reasoning":     "Needs compilation",
					},
				}},
			},
		},
	}

	emitter := &coordMockEmitter{}
	flowEngine := session.NewFlowEngine(logger, workspaceService, nil, emitter)
	digestManager := session.NewDigestManager(logger, client, session.DigestConfig{})
	coordinator := session.NewCoordinator(logger, workspaceService, client, emitter, flowEngine, &coordMockFlow{}, digestManager, "concept", "")

	ui := &coordMockUI{transitionToReturn: true, reviewToReturn: true}

	err := coordinator.RunAgentLoop(ctx, chatObj, activeSession, &coordMockThinkSpace{}, cognition.ThinkBundle{}, ui, nil)

	if err == nil {
		t.Fatalf("Expected TransitionRequest error, got nil")
	}

	transReq, ok := err.(*session.TransitionRequest)
	if !ok {
		t.Fatalf("Expected error of type *TransitionRequest, got %T: %v", err, err)
	}

	if transReq.TargetDomain != "golang-space" {
		t.Errorf("Expected transition to 'golang-space', got '%s'", transReq.TargetDomain)
	}

	if ui.capturedReasoning != "Needs compilation" {
		t.Errorf("Expected reasoning 'Needs compilation', got '%s'", ui.capturedReasoning)
	}
}
