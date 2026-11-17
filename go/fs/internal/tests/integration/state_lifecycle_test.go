package integration_test

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dgraph-io/badger/v4"
	"uuid"

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

type mockSandbox struct{}

func (m *mockSandbox) ResolvePath(path string) string                                { return path }
func (m *mockSandbox) WriteFile(ctx context.Context, path string, data []byte) error { return nil }
func (m *mockSandbox) ReadFile(ctx context.Context, path string) ([]byte, error) {
	return []byte(`{"trace": "ok"}`), nil
}
func (m *mockSandbox) ExecuteCommand(ctx context.Context, command string, args ...string) (string, error) {
	return "", nil
}
func (m *mockSandbox) ApplyDraft(ctx context.Context, message string) error { return nil }
func (m *mockSandbox) DeliverForReview(ctx context.Context) error           { return nil }
func (m *mockSandbox) TearDown(ctx context.Context) error                   { return nil }

type mockChatEngine struct {
	mu       sync.Mutex
	accepted []string
	rejected map[string]string
}

func (engine *mockChatEngine) InitChat(ctx context.Context, chatID string) error { return nil }
func (engine *mockChatEngine) Snapshot(ctx context.Context, chatID, msg string) (string, error) {
	return "mock-sha", nil
}
func (engine *mockChatEngine) SpawnCandidateSandbox(ctx context.Context, chatID, candidateID string) (workspace.CandidateSandbox, error) {
	return &mockSandbox{}, nil
}
func (engine *mockChatEngine) PreviewCandidate(ctx context.Context, chatID, candidateID string) error {
	return nil
}
func (engine *mockChatEngine) Accept(ctx context.Context, chatID, candidateID, reason string) error {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.accepted = append(engine.accepted, candidateID)
	return nil
}
func (engine *mockChatEngine) Reject(ctx context.Context, chatID, candidateID, reason string) error {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if engine.rejected == nil {
		engine.rejected = make(map[string]string)
	}
	engine.rejected[candidateID] = reason
	return nil
}
func (engine *mockChatEngine) ReadCandidateDiff(ctx context.Context, chatID, candidateID string) (string, error) {
	return "", nil
}

type mockDeterministicLLM struct {
	Responses []llm.StreamChunk
	callIndex int
}

func (m *mockDeterministicLLM) GenerateContentStream(
	ctx context.Context,
	model string,
	bundle cognition.ThinkBundle,
	registry *tools.Registry,
	config spaces.ThinkStateConfig,
) iter.Seq2[llm.StreamChunk, error] {
	return func(yield func(llm.StreamChunk, error) bool) {
		if m.callIndex < len(m.Responses) {
			yield(m.Responses[m.callIndex], nil)
			m.callIndex++
		}
	}
}

type mockThinkState struct{}

func (m *mockThinkState) Config() spaces.ThinkStateConfig {
	cfg := spaces.ThinkStateConfig{
		Timeouts: spaces.TimeoutsConfig{TurnSeconds: 10},
		Roles: spaces.RolesConfig{
			Manager: spaces.ManagerConfig{Model: "mock-manager"},
			Worker:  spaces.WorkerConfig{Model: "mock-worker"},
		},
	}
	cfg.ApplyDefaults()
	return cfg
}
func (m *mockThinkState) Verifier() spaces.Verifier { return m }
func (m *mockThinkState) Verify(ctx context.Context, sandbox workspace.CandidateSandbox) error {
	return nil
}
func (m *mockThinkState) Mapbook() spaces.Mapbook { return m }
func (m *mockThinkState) GenerateLayers(ctx context.Context, workspaceRoot string) (map[spaces.MapbookLayer]string, error) {
	return map[spaces.MapbookLayer]string{}, nil
}
func (m *mockThinkState) Patcher() workspace.Patcher                             { return nil }
func (m *mockThinkState) Initialize(ctx context.Context, directory string) error { return nil }
func (m *mockThinkState) EnrichBundle(ctx context.Context, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus, workspaceRoot string) error {
	return nil
}

type mockUI struct{}

func (ui *mockUI) OnTextChunk(chunk workspace.NodeChunk)                                  {}
func (ui *mockUI) StreamGraphEvent(address *thinkgraph.ChatAddress, node thinkgraph.Node) {}
func (ui *mockUI) ReviewContext(turnContext session.TurnContext) session.ContextApprovalDecision {
	return session.ContextApprovalDecision{Approved: true}
}
func (ui *mockUI) ChooseNextStep() (flows.FlowStrategy, bool) { return flows.StrategyRefine, false }
func (ui *mockUI) ReviewCandidate(branch string) (bool, string) {
	return true, "verified and accepted"
}
func (ui *mockUI) ConfirmTransition(targetThinkState string, reasoning string) bool { return true }
func (ui *mockUI) GetChunkChannel() chan<- workspace.NodeChunk                      { return nil }

type mockFlow struct{ step int }

func (f *mockFlow) Name() string  { return "MockFlow" }
func (f *mockFlow) MaxSteps() int { return 5 }
func (f *mockFlow) Next(state flows.FlowState) flows.FlowDirective {
	if f.step == 0 {
		f.step++
		return flows.FlowDirective{
			Action: flows.ActionSpawnAgents,
			Tasks:  []flows.SubAgentTask{{AgentID: "1", Instruction: "mock"}},
		}
	}
	if f.step == 1 {
		f.step++
		return flows.FlowDirective{Action: flows.ActionRequestStrategy}
	}
	if f.step == 2 {
		f.step++
		return flows.FlowDirective{
			Action:     flows.ActionSynthesize,
			AutoAccept: state.AutoAccept,
			Tasks:      []flows.SubAgentTask{{Instruction: "synthesize logic"}},
		}
	}
	return flows.FlowDirective{Action: flows.ActionFinish}
}

type mockEmitter struct{}

func (m *mockEmitter) Emit(channel string, event telemetry.Event) {}

func TestEndToEndStateLifecycle(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()
	spaceID := "integration-space"
	chatID := "lifecycle-chat"

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("Failed to init BadgerDB: %v", err)
	}
	defer graphDB.Close()

	registry := config.NewRegistry()
	if err := registry.LoadFS(fstest.MapFS{
		"fanout.yaml": &fstest.MapFile{
			Data: []byte(`
fanout:
  version: 3
  type: "flow"
  name: "Parallel FanOut"
  retry_prompt: "Retry: {{.ErrorTrace}}"
  review_prompt: "Review: {{ range .Branches }}{{ .CandidateID }}{{ end }}"
  refine_prompt: "Refine: {{ range .Branches }}{{ .CandidateID }}{{ end }}"
  synthesize_prompt: "Synthesize a single, unified implementation combining the strongest architectural patterns from the candidate branches."
  synthesis_review_prompt: "Summarize the synthesized candidate {{ .CandidateID }} for user approval."
`),
		},
		"digest.yaml": &fstest.MapFile{
			Data: []byte(`
digest:
  version: 1
  type: "core"
  name: "Digest Manager"
  manual_prompt: "Topic: {{.TopicName}}\nInstruction: {{.Instruction}}\nContext: {{.Context}}"
  background_prompt: "Target Window:\n{{.TargetWindow}}\nActive Buffer:\n{{.ActiveBuffer}}"
`),
		},
	}, "."); err != nil {
		t.Fatalf("Failed loading flow/core configs: %v", err)
	}

	var digestCfg session.DigestConfig
	if err := registry.DecodeCoreConfig("digest", &digestCfg); err != nil {
		t.Fatalf("Failed decoding digest config: %v", err)
	}

	mockSpace := &mockThinkState{}
	registry.RegisterThinkState("golang", mockSpace)
	registry.RegisterConfig("golang", mockSpace.Config())

	versioner := &mockChatEngine{}
	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(s string) workspace.ChatVersioner { return versioner }, graphDB, nil)
	if err := os.MkdirAll(filepath.Join(workspaceRoot, spaceID), 0755); err != nil {
		t.Fatalf("Failed creating space dir: %v", err)
	}
	if err := svcManager.UpdateSpaceState(spaceID, func(s *workspace.SpaceState) { s.ThinkState = "golang" }); err != nil {
		t.Fatalf("Failed updating space state: %v", err)
	}

	graphID := thinkgraph.NewGraphID(thinkgraph.PrefixChat)
	metaJSON := fmt.Sprintf(`{"id":"%s", "name":"Lifecycle Chat", "active_branch": "main"}`, chatID)

	chatDir := filepath.Join(workspaceRoot, spaceID, "chats", chatID)
	if err := os.MkdirAll(chatDir, 0755); err != nil {
		t.Fatalf("Failed creating chat dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chatDir, "meta.json"), []byte(metaJSON), 0644); err != nil {
		t.Fatalf("Failed writing meta.json: %v", err)
	}

	svc := svcManager.GetService(spaceID)
	chatAddress := &thinkgraph.ChatAddress{
		ChatID: chatID, ProjectID: spaceID, SubGraph: thinkgraph.SubGraph{Name: "main", GraphID: graphID},
	}
	if err := svc.InitializeChat(ctx, chatAddress); err != nil {
		t.Fatalf("Failed to initialize chat: %v", err)
	}

	var targetEvents []thinkgraph.Event
	var integrationChatSession *session.ChatSession

	t.Run("Act I: The Conversation", func(t *testing.T) {
		if err := svc.AppendUserPrompt(ctx, chatAddress, "How do I build a server?"); err != nil {
			t.Fatalf("LogUserPrompt failed: %v", err)
		}
		time.Sleep(1 * time.Millisecond)
		if err := svc.AppendModelResponse(ctx, chatAddress, uuid.NewV7(), "Use net/http.", nil, uuid.Nil()); err != nil {
			t.Fatalf("LogModelResponse failed: %v", err)
		}

		timeline, err := graphDB.GetSubGraphTimeline(ctx, chatAddress, chatAddress.GraphID)
		if err != nil {
			t.Fatalf("GetSubGraphTimeline failed: %v", err)
		}
		if len(timeline) != 2 {
			t.Fatalf("Expected 2 events in graph, got %d", len(timeline))
		}

		for _, n := range timeline {
			targetEvents = append(targetEvents, n.Event)
		}
	})

	t.Run("Act II: The Compression", func(t *testing.T) {
		mockJSON := `{
			"thought": "Summarizing the server setup",
			"digests": [
				{
					"topic_name": "Server Setup",
					"tags": ["golang", "http"],
					"summary": "User asked how to build a server. Model suggested net/http."
				}
			]
		}`

		mockLLM := &mockDeterministicLLM{Responses: []llm.StreamChunk{{Text: mockJSON}}}
		digestManager := session.NewDigestManager(logger, mockLLM, digestCfg)

		count, err := digestManager.CreateBackgroundDigest(ctx, svc, chatAddress, targetEvents, nil, true, "mock-model", "persona")
		if err != nil || count != 1 {
			t.Fatalf("Failed to create digest: %v", err)
		}

		graphDigests, err := graphDB.GetDigests(ctx, chatAddress)
		if err != nil {
			t.Fatalf("GetDigests failed: %v", err)
		}
		if len(graphDigests) != 1 {
			t.Fatalf("Expected 1 digest in graph, got %d", len(graphDigests))
		}
	})

	t.Run("Act III: Amnesia & Rehydration", func(t *testing.T) {
		if err := graphDB.ClearChatGraph(ctx, chatAddress); err != nil {
			t.Fatalf("ClearChatGraph failed: %v", err)
		}

		rehydrator := thinkgraph.NewRehydrator(svc.Dispatcher().GraphStore(), workspaceRoot)
		if err := rehydrator.Rebuild(ctx, chatAddress); err != nil {
			t.Fatalf("Rehydration failed: %v", err)
		}

		restoredTimeline, err := graphDB.GetSubGraphTimeline(ctx, chatAddress, chatAddress.GraphID)
		if err != nil {
			t.Fatalf("GetSubGraphTimeline after rehydration failed: %v", err)
		}
		if len(restoredTimeline) != 2 {
			t.Fatalf("Expected 2 events restored, got %d", len(restoredTimeline))
		}

		restoredDigests, err := graphDB.GetDigests(ctx, chatAddress)
		if err != nil {
			t.Fatalf("GetDigests after rehydration failed: %v", err)
		}
		if len(restoredDigests) != 1 {
			t.Fatalf("Expected 1 digest restored, got %d", len(restoredDigests))
		}
	})

	t.Run("Act IV: Tool Delegation & Synthesize Flow Resolution", func(t *testing.T) {
		toolLLM := &mockDeterministicLLM{
			Responses: []llm.StreamChunk{
				{
					ToolCalls: []llm.ToolCall{{
						Name: "propose_change",
						Args: map[string]any{
							"instruction":    "Write tests",
							"context_digest": "Testing context",
							"target_files":   []any{},
							"agent_count":    1,
							"agent_instructions": []any{
								map[string]any{
									"approach":    "standard",
									"instruction": "Write tests",
								},
							},
						},
					}},
				},
				{
					Text: `{"thought":"Sub-agent branch output","patches":[]}`,
				},
				{
					Text: "Manager evaluation: Combine the best test patterns into a unified test suite.",
				},
				{
					Text: `{"thought":"Golden Synthesis Patch","patches":[]}`,
				},
				{
					Text: "Final synthesized test suite summary on main timeline.",
				},
			},
		}

		emitter := &mockEmitter{}
		ui := &mockUI{}
		toolRegistry := tools.NewRegistry()
		toolRegistry.Register(tools.NewProposeChangeTool())

		turnExecutor := session.NewTurnExecutor(logger, toolLLM, svc)

		responseID := uuid.NewV7()

		signal, err := turnExecutor.Execute(ctx, chatAddress, responseID, uuid.Nil(), "mock-manager", false, mockSpace, cognition.ThinkBundle{}, toolRegistry, ui, emitter)
		if err != nil {
			t.Fatalf("TurnExecutor failed: %v", err)
		}

		dispatchSig, ok := signal.(*signals.SignalDispatchFlow)
		if !ok {
			t.Fatalf("Expected TurnExecutor to yield a SignalDispatchFlow, got %T", signal)
		}

		digestManager := session.NewDigestManager(logger, toolLLM, digestCfg)
		integrationChatSession, err = session.NewChatSession(ctx, logger, registry, svcManager, toolLLM, &mockFlow{}, digestManager, spaceID, chatID)
		if err != nil {
			t.Fatalf("Failed to spin up ChatSession supervisor: %v", err)
		}

		workerEngine := session.NewWorkerEngine(logger, integrationChatSession, registry, svc, turnExecutor, svc.Dispatcher(), &mockFlow{})
		synthEngine := session.NewSynthesizeEngine(logger, registry, svc, turnExecutor, integrationChatSession)

		workerSig, err := workerEngine.Execute(ctx, chatAddress, responseID, uuid.NewV7(), dispatchSig, ui, emitter)
		if err != nil {
			t.Fatalf("WorkerEngine failed: %v", err)
		}

		synthSig, ok := workerSig.(*signals.SignalSynthesize)
		if !ok {
			t.Fatalf("Expected WorkerEngine to yield a SignalSynthesize, got %T", workerSig)
		}

		finalSig, err := synthEngine.Execute(ctx, chatAddress, responseID, uuid.NewV7(), synthSig, ui, emitter)
		if err != nil {
			t.Fatalf("SynthesizeEngine failed: %v", err)
		}

		if !finalSig.IsTerminal() {
			t.Fatalf("Expected SynthesizeEngine to yield a Terminal signal")
		}

		flowAddr := synthSig.FlowAddress
		expectedSynthCand := fmt.Sprintf("%s-%s-synthesis", chatID, flowAddr.SubGraph.Name)
		expectedWorkerCand := fmt.Sprintf("%s-%s-1", chatID, flowAddr.SubGraph.Name)

		if len(versioner.accepted) != 1 || versioner.accepted[0] != expectedSynthCand {
			t.Errorf("Expected synthesis candidate %q to be accepted, got %v", expectedSynthCand, versioner.accepted)
		}
		if versioner.rejected[expectedWorkerCand] != "Superseded by synthesis" {
			t.Errorf("Expected superseded worker candidate %q to be rejected, got %q", expectedWorkerCand, versioner.rejected[expectedWorkerCand])
		}

		if err := svc.SyncGraphContext(ctx, flowAddr); err != nil {
			t.Fatalf("SyncGraphContext failed: %v", err)
		}
		flowTimeline, err := svc.Graph().GetSubGraphTimeline(ctx, flowAddr, flowAddr.GraphID)
		if err != nil {
			t.Fatalf("GetSubGraphTimeline failed: %v", err)
		}

		hasRefinePrompt := false
		hasSynthesizeTaskPrompt := false
		hasSynthRes := false
		for _, n := range flowTimeline {
			if n.Type == thinkgraph.EventPrompt && n.Edges["parent_id"] == synthSig.ParentNodeID.String() {
				hasRefinePrompt = true
			}
			if n.Type == thinkgraph.EventPrompt && n.Metadata["agent_id"] == "synthesis" {
				hasSynthesizeTaskPrompt = true
			}
			if n.Type == workspace.EventCandidateResolution && n.Metadata["agent_id"] == "synthesis" && strings.Contains(n.Content, "ACCEPTED") {
				hasSynthRes = true
			}
		}

		if !hasRefinePrompt {
			t.Errorf("Expected Manager refine prompt node linked to ParentNodeID %s across boundary", synthSig.ParentNodeID.String())
		}
		if !hasSynthesizeTaskPrompt {
			t.Errorf("Expected synthesis worker prompt node from YAML synthesize_prompt on flow subgraph")
		}
		if !hasSynthRes {
			t.Errorf("Expected synthesis candidate resolution ACCEPTED node at the tail of the flow subgraph")
		}

		mainTimeline, err := svc.Graph().GetSubGraphTimeline(ctx, chatAddress, chatAddress.GraphID)
		if err != nil {
			t.Fatalf("GetSubGraphTimeline failed: %v", err)
		}
		var managerReviewID uuid.UUID
		hasResolution := false
		var userDecisionParentID string
		for _, n := range mainTimeline {
			if n.Type == thinkgraph.EventModel && n.Content == "Final synthesized test suite summary on main timeline." {
				managerReviewID = n.ID
			}
			if n.Type == workspace.EventCandidateResolution {
				hasResolution = true
			}
			if n.Type == thinkgraph.EventPrompt && strings.Contains(n.Content, "accepted flow: verified and accepted") {
				userDecisionParentID = n.Edges["parent_id"]
			}
		}
		if managerReviewID == uuid.Nil() {
			t.Errorf("Expected Manager final review EventModel node on the main timeline")
		}
		if !hasResolution {
			t.Errorf("Expected CandidateResolution node on the main timeline")
		}
		if userDecisionParentID != managerReviewID.String() {
			t.Errorf("Expected user decision EventPrompt node to link to Manager review node %s, got %q", managerReviewID.String(), userDecisionParentID)
		}
	})

	t.Run("Act V: System Transition & Turn Reboot", func(t *testing.T) {
		if integrationChatSession == nil {
			t.Skip("Skipping Act V because integrationChatSession failed to initialize in Act IV")
		}

		registry.RegisterThinkState("concept", mockSpace)
		registry.RegisterConfig("concept", mockSpace.Config())

		catalystEvent := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Switch to concept mode")
		if err := svc.Dispatcher().AppendEvent(ctx, chatAddress, catalystEvent); err != nil {
			t.Fatalf("Failed to seed catalyst prompt event: %v", err)
		}

		systemEngine := session.NewSystemEngine(logger, integrationChatSession, registry, svc)

		transitionSig := &signals.SignalTransition{
			TargetThinkState: "concept",
		}

		turnID := uuid.NewV7()
		nextSignal, err := systemEngine.Execute(ctx, chatAddress, turnID, catalystEvent.ID, transitionSig, &mockUI{}, &mockEmitter{})
		if err != nil {
			t.Fatalf("SystemEngine failed: %v", err)
		}

		if _, ok := nextSignal.(*signals.SignalExecuteChat); !ok {
			t.Fatalf("Expected SystemEngine to yield SignalExecuteChat to reboot the turn, got %T", nextSignal)
		}

		meta, err := svcManager.GetChatMeta(spaceID, chatID)
		if err != nil {
			t.Fatalf("Failed to fetch chat meta: %v", err)
		}
		if meta.ActiveThinkState != "concept" {
			t.Errorf("Expected active think state to be explicitly mutated to 'concept', got '%s'", meta.ActiveThinkState)
		}

		mainTimeline, err := svc.Graph().GetSubGraphTimeline(ctx, chatAddress, chatAddress.GraphID)
		if err != nil {
			t.Fatalf("Failed to fetch main timeline: %v", err)
		}
		foundTransitionBadge := false
		for _, n := range mainTimeline {
			if n.ID == catalystEvent.ID && n.Metadata["transitioned_to"] == "concept" {
				foundTransitionBadge = true
				break
			}
		}
		if !foundTransitionBadge {
			t.Errorf("Expected catalyst prompt node %s to be mutated with transitioned_to='concept'", catalystEvent.ID)
		}

		sysAddr := integrationChatSession.GetSystemAddress()
		sysTimeline, err := svc.Graph().GetSubGraphTimeline(ctx, sysAddr, sysAddr.GraphID)
		if err != nil {
			t.Fatalf("Failed to fetch system timeline: %v", err)
		}
		foundSystemEvent := false
		for _, n := range sysTimeline {
			if n.Type == thinkgraph.EventSystem && strings.Contains(n.Content, "Workspace transitioned to concept") {
				foundSystemEvent = true
				break
			}
		}
		if !foundSystemEvent {
			t.Errorf("Expected transition EventSystem node to be appended to 'system' subgraph timeline")
		}
	})
}
