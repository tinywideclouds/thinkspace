package session_test

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"os"
	"sync"
	"testing"

	"uuid"

	"github.com/dgraph-io/badger/v4"
	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/db"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces/golang"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

func TestSubAgentRunner_RetryAndVerificationSuccess(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	engine := &weMockChatEngine{}
	workspaceRoot := t.TempDir()

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("failed to init badger: %v", err)
	}
	defer graphDB.Close()

	emitter := &weMockEmitter{}
	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatVersioner { return engine }, graphDB, emitter)
	service := svcManager.GetService("golang")

	chatAddr := &thinkgraph.ChatAddress{
		ChatID:    "chat-runner",
		ProjectID: "golang",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}
	if err := service.InitializeChat(ctx, chatAddr); err != nil {
		t.Fatalf("InitializeChat failed: %v", err)
	}

	flowID := uuid.NewV7()
	flowAddr := &thinkgraph.ChatAddress{
		ChatID:    "chat-runner",
		ProjectID: "golang",
		SubGraph: thinkgraph.SubGraph{
			Name:    "flow-" + flowID.String(),
			GraphID: thinkgraph.GraphID{Prefix: thinkgraph.PrefixFlow, UUID: flowID},
		},
	}

	mockSpace := &weMockThinkSpace{
		errsToReturn: []error{errors.New("syntax error on attempt 1")},
	}
	modelClient := &weMockModelClient{
		responses: []llm.StreamChunk{
			{Text: "attempt 1 output"},
			{Text: "attempt 2 output"},
		},
	}

	turnExecutor := session.NewTurnExecutor(logger, modelClient, service)
	runner := session.NewSubAgentRunner(logger, service, turnExecutor, nil, nil)
	scope := session.NewFlowScope(logger, service, nil, chatAddr, flowAddr, flowID)
	ui := &weMockUI{}

	res := runner.Run(ctx, session.SubAgentRunRequest{
		ChatSession: chatAddr,
		FlowAddress: flowAddr,
		Scope:       scope,
		Space:       mockSpace,
		FlowContext: flows.FlowContext{
			FlowID:         "flow-" + flowID.String(),
			SpaceID:        "golang",
			BaseAgentRules: "You are a Go worker",
		},
		Task: flows.SubAgentTask{
			AgentID:       "agent-1",
			ContextDigest: "Digest",
			Instruction:   "Implement feature",
			RetryPrompt:   "Fix error: {{.ErrorTrace}}",
		},
		AgentIndex:   1,
		ParentNodeID: uuid.NewV7(),
		UI:           ui,
		Emitter:      emitter,
	})

	if res.Err != nil {
		t.Fatalf("unexpected runner error: %v", res.Err)
	}
	if !res.Passed {
		t.Errorf("expected SubAgentRunner to pass on retry attempt 2")
	}
	expectedBareID := fmt.Sprintf("chat-runner-flow-%s-agent-1", flowID.String())
	if res.Candidate.ID != expectedBareID || res.CandidateID != expectedBareID {
		t.Errorf("expected bare canonical CandidateID %q, got %q", expectedBareID, res.Candidate.ID)
	}
	if res.Candidate.BranchRef() != "candidate/"+expectedBareID {
		t.Errorf("expected BranchRef %q, got %q", "candidate/"+expectedBareID, res.Candidate.BranchRef())
	}
	if len(scope.Candidates()) != 1 || scope.Candidates()[0].ID != expectedBareID {
		t.Errorf("expected candidate to be registered in FlowScope")
	}
	if !engine.sandbox.delivered {
		t.Errorf("expected candidate sandbox to be delivered for review")
	}
	if res.Record.StateDelta != "+ mock diff" {
		t.Errorf("expected candidate diff to be populated on record, got %q", res.Record.StateDelta)
	}
	if len(res.LeafEventIDs) != 1 {
		t.Errorf("expected 1 resolution leaf node ID, got %d", len(res.LeafEventIDs))
	}
}

type modelCapturingClient struct {
	capturedModels []string
	responses      []llm.StreamChunk
	idx            int
}

func (c *modelCapturingClient) GenerateContentStream(
	ctx context.Context,
	model string,
	bundle cognition.ThinkBundle,
	registry *tools.Registry,
	config spaces.ThinkStateConfig,
) iter.Seq2[llm.StreamChunk, error] {
	c.capturedModels = append(c.capturedModels, model)
	return func(yield func(llm.StreamChunk, error) bool) {
		if c.idx < len(c.responses) {
			resp := c.responses[c.idx]
			c.idx++
			yield(resp, nil)
		}
	}
}

type realPatchingSpace struct {
	cfg     spaces.ThinkStateConfig
	patcher workspace.Patcher
}

func (s *realPatchingSpace) Config() spaces.ThinkStateConfig                        { return s.cfg }
func (s *realPatchingSpace) Verifier() spaces.Verifier                              { return nil }
func (s *realPatchingSpace) Mapbook() spaces.Mapbook                                { return s }
func (s *realPatchingSpace) Patcher() workspace.Patcher                             { return s.patcher }
func (s *realPatchingSpace) Initialize(ctx context.Context, directory string) error { return nil }
func (s *realPatchingSpace) GenerateLayers(ctx context.Context, workspaceRoot string) (map[spaces.MapbookLayer]string, error) {
	return map[spaces.MapbookLayer]string{}, nil
}
func (s *realPatchingSpace) EnrichBundle(ctx context.Context, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus, workspaceRoot string) error {
	return nil
}

func TestSubAgentRunner_UsesWorkerModelAndHealsBackslashSpace(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	engine := &weMockChatEngine{}
	workspaceRoot := t.TempDir()

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("failed to init badger: %v", err)
	}
	defer graphDB.Close()

	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatVersioner { return engine }, graphDB, nil)
	service := svcManager.GetService("golang")

	chatAddr := &thinkgraph.ChatAddress{
		ChatID:    "chat-worker-model",
		ProjectID: "golang",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}
	if err := service.InitializeChat(ctx, chatAddr); err != nil {
		t.Fatalf("InitializeChat failed: %v", err)
	}

	flowID := uuid.NewV7()
	flowAddr := &thinkgraph.ChatAddress{
		ChatID:    "chat-worker-model",
		ProjectID: "golang",
		SubGraph: thinkgraph.SubGraph{
			Name:    "flow-" + flowID.String(),
			GraphID: thinkgraph.GraphID{Prefix: thinkgraph.PrefixFlow, UUID: flowID},
		},
	}

	cfg := spaces.ThinkStateConfig{
		Name: "golang",
		Roles: spaces.RolesConfig{
			Manager: spaces.ManagerConfig{Model: "gemini-3.8-flash"},
			Worker:  spaces.WorkerConfig{Model: "gemini-4-hydrogen"},
		},
	}
	cfg.ApplyDefaults()

	space := &realPatchingSpace{
		cfg:     cfg,
		patcher: golang.NewGoASTPatcher("", false),
	}

	// Simulate gemini-4-hydrogen response with an invalid "\ " JSON escape
	client := &modelCapturingClient{
		responses: []llm.StreamChunk{
			{Text: `{"thought":"Writing server","patches":[{"file":"src/main.go","action":"full_replace","code":"package main\n// comment\ with space\nfunc main() {}"}]}`},
		},
	}

	turnExecutor := session.NewTurnExecutor(logger, client, service)
	runner := session.NewSubAgentRunner(logger, service, turnExecutor, nil, nil)
	ui := &weMockUI{}

	res := runner.Run(ctx, session.SubAgentRunRequest{
		ChatSession: chatAddr,
		FlowAddress: flowAddr,
		Space:       space,
		FlowContext: flows.FlowContext{
			FlowID:  "flow-" + flowID.String(),
			SpaceID: "golang",
		},
		Task: flows.SubAgentTask{
			AgentID:     "agent-1",
			Instruction: "Build server",
		},
		AgentIndex:   1,
		ParentNodeID: uuid.NewV7(),
		UI:           ui,
	})

	if res.Err != nil {
		t.Fatalf("unexpected runner error: %v", res.Err)
	}
	if !res.PatchApplied || !res.Passed {
		t.Fatalf("expected healer + patcher to succeed, got patchApplied=%v passed=%v trace=%q", res.PatchApplied, res.Passed, res.Trace)
	}
	if len(client.capturedModels) != 1 || client.capturedModels[0] != "gemini-4-hydrogen" {
		t.Errorf("expected worker turn to use Roles.Worker.Model 'gemini-4-hydrogen', got %v", client.capturedModels)
	}
}

type concurrentSafeModelClient struct{}

func (c *concurrentSafeModelClient) GenerateContentStream(
	ctx context.Context,
	model string,
	bundle cognition.ThinkBundle,
	registry *tools.Registry,
	config spaces.ThinkStateConfig,
) iter.Seq2[llm.StreamChunk, error] {
	return func(yield func(llm.StreamChunk, error) bool) {
		yield(llm.StreamChunk{Text: "concurrent output"}, nil)
	}
}

type concurrentSafeChatEngine struct{}

func (m *concurrentSafeChatEngine) InitChat(ctx context.Context, chatID string) error { return nil }
func (m *concurrentSafeChatEngine) Snapshot(ctx context.Context, chatID string, msg string) (string, error) {
	return "", nil
}
func (m *concurrentSafeChatEngine) PreviewCandidate(ctx context.Context, chatID string, candidateID string) error {
	return nil
}
func (m *concurrentSafeChatEngine) ReadCandidateDiff(ctx context.Context, chatID string, candidateID string) (string, error) {
	return "+ concurrent diff", nil
}
func (m *concurrentSafeChatEngine) Accept(ctx context.Context, chatID string, candidateID string, reason string) error {
	return nil
}
func (m *concurrentSafeChatEngine) Reject(ctx context.Context, chatID string, candidateID string, reason string) error {
	return nil
}
func (m *concurrentSafeChatEngine) SpawnCandidateSandbox(ctx context.Context, chatID string, candidateID string) (workspace.CandidateSandbox, error) {
	return &weMockSandbox{candidateID: candidateID}, nil
}

type threadSafeEmitter struct {
	mu     sync.Mutex
	events []telemetry.Event
}

func (e *threadSafeEmitter) Emit(channel string, event telemetry.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, event)
}

func TestSubAgentRunner_ConcurrentRaceSafety(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("failed to init badger: %v", err)
	}
	defer graphDB.Close()

	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatVersioner {
		return &concurrentSafeChatEngine{}
	}, graphDB, nil)
	service := svcManager.GetService("golang")

	chatAddr := &thinkgraph.ChatAddress{
		ChatID:    "chat-race",
		ProjectID: "golang",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}
	if err := service.InitializeChat(ctx, chatAddr); err != nil {
		t.Fatalf("InitializeChat failed: %v", err)
	}

	flowID := uuid.NewV7()
	flowAddr := &thinkgraph.ChatAddress{
		ChatID:    "chat-race",
		ProjectID: "golang",
		SubGraph: thinkgraph.SubGraph{
			Name:    "flow-" + flowID.String(),
			GraphID: thinkgraph.GraphID{Prefix: thinkgraph.PrefixFlow, UUID: flowID},
		},
	}

	turnExecutor := session.NewTurnExecutor(logger, &concurrentSafeModelClient{}, service)
	runner := session.NewSubAgentRunner(logger, service, turnExecutor, nil, nil)
	scope := session.NewFlowScope(logger, service, nil, chatAddr, flowAddr, flowID)

	const numAgents = 4
	var wg sync.WaitGroup
	results := make([]session.SubAgentResult, numAgents)

	for i := 0; i < numAgents; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			agentEmitter := &threadSafeEmitter{}
			results[idx] = runner.Run(ctx, session.SubAgentRunRequest{
				ChatSession: chatAddr,
				FlowAddress: flowAddr,
				Scope:       scope,
				Space:       &weMockThinkSpace{},
				FlowContext: flows.FlowContext{
					FlowID:         "flow-" + flowID.String(),
					SpaceID:        "golang",
					BaseAgentRules: "You are a Go worker",
				},
				Task: flows.SubAgentTask{
					AgentID:     fmt.Sprintf("agent-%d", idx+1),
					Instruction: "Concurrent task",
				},
				AgentIndex:   idx + 1,
				ParentNodeID: uuid.NewV7(),
				Emitter:      agentEmitter,
			})
		}(i)
	}
	wg.Wait()

	for i, res := range results {
		if res.Err != nil {
			t.Errorf("agent %d failed: %v", i+1, res.Err)
		}
		if !res.Passed {
			t.Errorf("agent %d expected to pass", i+1)
		}
	}
	if len(scope.Candidates()) != numAgents {
		t.Errorf("expected all %d concurrent candidates registered in FlowScope, got %d", numAgents, len(scope.Candidates()))
	}
}
