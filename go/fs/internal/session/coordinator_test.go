package session_test

import (
	"context"
	"iter"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/assembler"
	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// --- Shared Mocks ---

type coordMockModelClient struct {
	responses      []llm.StreamChunk
	callIndex      int
	capturedBundle cognition.ThinkBundle // Fixed undefined error here
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

func (m *coordMockThinkSpace) Config() spaces.ThinkSpaceConfig {
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
func (m *coordMockThinkSpace) Tools() []llm.ToolDef                                   { return nil }
func (m *coordMockThinkSpace) Verifier() spaces.Verifier                              { return nil }
func (m *coordMockThinkSpace) Mapbook() assembler.Mapbook                             { return nil }
func (m *coordMockThinkSpace) Patcher() workspace.Patcher                             { return nil }
func (m *coordMockThinkSpace) Initialize(ctx context.Context, directory string) error { return nil }

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

	// Phase 2 Pruning
	retainedFiles   []string
	retainedHistory []int
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

// --- Tests ---

func TestCoordinator_RunAgentLoop_RoutesToEngine(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()
	bus := chat.NewEventBus()
	bus.Subscribe(chat.NewLedgerSubscriber())

	workspaceService := workspace.NewService(logger, &coordMockChatEngine{}, workspaceRoot, bus)
	thread, _ := workspaceService.StartThread(ctx, "test-thread")

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
	manifest := chat.NewThreadManifest(thread.ID)

	err := coordinator.RunAgentLoop(ctx, thread, manifest, &coordMockThinkSpace{}, cognition.ThinkBundle{}, ui, nil)
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
	bus := chat.NewEventBus()
	workspaceService := workspace.NewService(logger, &coordMockChatEngine{}, workspaceRoot, bus)
	thread, _ := workspaceService.StartThread(ctx, "test-thread-lens")

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

	manifest := chat.NewThreadManifest(thread.ID)
	_ = coordinator.RunAgentLoop(ctx, thread, manifest, &coordMockThinkSpace{}, cognition.ThinkBundle{}, &coordMockUI{reviewToReturn: true}, nil)

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

func TestCoordinator_CreateDigest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()
	bus := chat.NewEventBus()
	bus.Subscribe(chat.NewLedgerSubscriber())

	workspaceService := workspace.NewService(logger, &coordMockChatEngine{}, workspaceRoot, bus)
	thread, _ := workspaceService.StartThread(ctx, "test-thread-digest")

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
				// This is the response returned by the mock client when DigestManager calls it
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

	manifest := chat.NewThreadManifest(thread.ID)
	_ = coordinator.RunAgentLoop(ctx, thread, manifest, &coordMockThinkSpace{}, cognition.ThinkBundle{}, &coordMockUI{reviewToReturn: true}, nil)

	events, _ := workspaceService.LoadEvents(ctx, thread)
	foundDigest := false
	for _, e := range events {
		if e.Type == chat.EventDigest && strings.Contains(e.Content, "Generated Manual Summary") {
			foundDigest = true
			if len(e.Tags) != 1 || e.Tags[0] != "auth" {
				t.Errorf("Expected tags to be passed into ledger correctly")
			}
			break
		}
	}

	if !foundDigest {
		t.Errorf("Expected create_digest tool to route execution to the DigestManager and write to ledger")
	}
}

func TestCoordinator_TransitionWorkspace_Approved(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()
	bus := chat.NewEventBus()
	workspaceService := workspace.NewService(logger, &coordMockChatEngine{}, workspaceRoot, bus)
	thread, _ := workspaceService.StartThread(ctx, "test-thread-trans")

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
	manifest := chat.NewThreadManifest(thread.ID)

	err := coordinator.RunAgentLoop(ctx, thread, manifest, &coordMockThinkSpace{}, cognition.ThinkBundle{}, ui, nil)

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
