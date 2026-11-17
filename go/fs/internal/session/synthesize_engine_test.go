package session_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

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

const testSynthesizePromptYAML = "Synthesize a single, unified implementation strictly following the Manager's comparative evaluation in your background context."

func registerFanoutFlowConfig(t *testing.T, registry *config.Registry) {
	t.Helper()
	mockFS := fstest.MapFS{
		"fanout.yaml": &fstest.MapFile{
			Data: []byte(fmt.Sprintf(`
fanout:
  version: 3
  type: "flow"
  name: "Parallel FanOut"
  retry_prompt: "Retry: {{.ErrorTrace}}"
  review_prompt: "Review: {{ range .Branches }}{{ .CandidateID }}{{ end }}"
  refine_prompt: "Refine: {{ range .Branches }}{{ .CandidateID }}{{ end }}"
  synthesize_prompt: %q
  synthesis_review_prompt: "Review synthesized proposal {{ .CandidateID }}"
`, testSynthesizePromptYAML)),
		},
	}
	if err := registry.LoadFS(mockFS, "."); err != nil {
		t.Fatalf("failed to load fanout.yaml into registry: %v", err)
	}
}

type seTestModelClient struct {
	responses              []llm.StreamChunk
	callIdx                int
	capturedModels         []string
	capturedPrompts        []string
	capturedLongTermMemory [][]string
}

func (client *seTestModelClient) GenerateContentStream(
	ctx context.Context,
	model string,
	bundle cognition.ThinkBundle,
	registry *tools.Registry,
	config spaces.ThinkStateConfig,
) iter.Seq2[llm.StreamChunk, error] {
	client.capturedModels = append(client.capturedModels, model)
	client.capturedPrompts = append(client.capturedPrompts, bundle.Trajectory.Catalyst.Content)
	client.capturedLongTermMemory = append(client.capturedLongTermMemory, append([]string(nil), bundle.Trajectory.LongTermMemory...))
	return func(yield func(llm.StreamChunk, error) bool) {
		if client.callIdx < len(client.responses) {
			resp := client.responses[client.callIdx]
			client.callIdx++
			yield(resp, nil)
		}
	}
}

type seTestChatEngine struct {
	mu                sync.Mutex
	sandbox           *seTestSandbox
	acceptCalled      bool
	acceptedCandidate string
	rejected          map[string]string
}

func (m *seTestChatEngine) InitChat(ctx context.Context, chatID string) error { return nil }
func (m *seTestChatEngine) Snapshot(ctx context.Context, chatID string, msg string) (string, error) {
	return "", nil
}
func (m *seTestChatEngine) PreviewCandidate(ctx context.Context, chatID string, candidateID string) error {
	return nil
}
func (m *seTestChatEngine) ReadCandidateDiff(ctx context.Context, chatID string, candidateID string) (string, error) {
	return "+ synthesized diff", nil
}
func (m *seTestChatEngine) Accept(ctx context.Context, chatID string, candidateID string, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.acceptCalled = true
	m.acceptedCandidate = candidateID
	return nil
}
func (m *seTestChatEngine) Reject(ctx context.Context, chatID string, candidateID string, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rejected == nil {
		m.rejected = make(map[string]string)
	}
	m.rejected[candidateID] = reason
	return nil
}
func (m *seTestChatEngine) SpawnCandidateSandbox(ctx context.Context, chatID string, candidateID string) (workspace.CandidateSandbox, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sandbox = &seTestSandbox{candidateID: candidateID}
	return m.sandbox, nil
}

type seTestSandbox struct {
	candidateID string
	delivered   bool
}

func (m *seTestSandbox) ResolvePath(path string) string                                { return path }
func (m *seTestSandbox) WriteFile(ctx context.Context, path string, data []byte) error { return nil }
func (m *seTestSandbox) ReadFile(ctx context.Context, path string) ([]byte, error) {
	return []byte("mock file content"), nil
}
func (m *seTestSandbox) ExecuteCommand(ctx context.Context, cmd string, args ...string) (string, error) {
	return "", nil
}
func (m *seTestSandbox) ApplyDraft(ctx context.Context, message string) error { return nil }
func (m *seTestSandbox) DeliverForReview(ctx context.Context) error           { m.delivered = true; return nil }
func (m *seTestSandbox) TearDown(ctx context.Context) error                   { return nil }

type seTestThinkSpace struct {
	verifyErr error
	patcher   workspace.Patcher
}

func (m *seTestThinkSpace) Initialize(ctx context.Context, directory string) error { return nil }
func (m *seTestThinkSpace) Config() spaces.ThinkStateConfig {
	cfg := spaces.ThinkStateConfig{
		Roles: spaces.RolesConfig{
			Manager: spaces.ManagerConfig{Model: "gemini-3.8-flash"},
			Worker:  spaces.WorkerConfig{Model: "gemini-4-hydrogen", RetryPrompt: "Retry: {{.ErrorTrace}}"},
		},
	}
	cfg.ApplyDefaults()
	return cfg
}
func (m *seTestThinkSpace) Patcher() workspace.Patcher { return m.patcher }
func (m *seTestThinkSpace) Verifier() spaces.Verifier  { return m }
func (m *seTestThinkSpace) Verify(ctx context.Context, sandbox workspace.CandidateSandbox) error {
	return m.verifyErr
}
func (m *seTestThinkSpace) Mapbook() spaces.Mapbook { return m }
func (m *seTestThinkSpace) GenerateLayers(ctx context.Context, root string) (map[spaces.MapbookLayer]string, error) {
	return map[spaces.MapbookLayer]string{}, nil
}
func (m *seTestThinkSpace) EnrichBundle(ctx context.Context, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus, workspaceRoot string) error {
	return nil
}

type seTestUI struct {
	reviewToReturn    bool
	reasonToReturn    string
	reviewCalls       []string
	streamedSubGraphs []thinkgraph.Node
	onReviewCandidate func(branch string)
}

func (u *seTestUI) OnTextChunk(chunk workspace.NodeChunk) {}
func (u *seTestUI) StreamGraphEvent(address *thinkgraph.ChatAddress, node thinkgraph.Node) {
	if address != nil && strings.HasPrefix(address.Name, "flow-") {
		u.streamedSubGraphs = append(u.streamedSubGraphs, node)
	}
}
func (u *seTestUI) ReviewContext(ctxData session.TurnContext) session.ContextApprovalDecision {
	return session.ContextApprovalDecision{}
}
func (u *seTestUI) ChooseNextStep() (flows.FlowStrategy, bool) { return flows.StrategyRefine, false }
func (u *seTestUI) ReviewCandidate(branch string) (bool, string) {
	u.reviewCalls = append(u.reviewCalls, branch)
	if u.onReviewCandidate != nil {
		u.onReviewCandidate(branch)
	}
	return u.reviewToReturn, u.reasonToReturn
}
func (u *seTestUI) ConfirmTransition(targetThinkState string, reasoning string) bool { return true }
func (u *seTestUI) GetChunkChannel() chan<- workspace.NodeChunk                      { return nil }

type seTestEmitter struct{}

func (m *seTestEmitter) Emit(channel string, event telemetry.Event) {}

func seedFlowNodeOnMain(t *testing.T, service *workspace.Service, chatSession *thinkgraph.ChatAddress, flowID uuid.UUID) {
	t.Helper()
	flowEv := thinkgraph.NewEvent(flows.EventFlowLifecycle, "Flow started")
	flowEv.ID = flowID
	if err := service.Dispatcher().AppendEvent(context.Background(), chatSession, flowEv); err != nil {
		t.Fatalf("failed to seed flow lifecycle node on main: %v", err)
	}
}

func TestSynthesizeEngine_TwoStageSubgraphTopologyAndManualReview(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	engine := &seTestChatEngine{}
	workspaceRoot := t.TempDir()

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("failed to init badger: %v", err)
	}
	defer graphDB.Close()

	emitter := &seTestEmitter{}
	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatVersioner { return engine }, graphDB, emitter)
	service := svcManager.GetService("golang")

	chatSession := &thinkgraph.ChatAddress{
		ChatID:    "chat-1",
		ProjectID: "golang",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}
	if err := service.InitializeChat(ctx, chatSession); err != nil {
		t.Fatalf("InitializeChat failed: %v", err)
	}

	registry := config.NewRegistry()
	registerFanoutFlowConfig(t, registry)
	mockSpace := &seTestThinkSpace{}
	registry.RegisterThinkState("golang", mockSpace)
	registry.RegisterConfig("golang", mockSpace.Config())
	if err := svcManager.UpdateSpaceState("golang", func(s *workspace.SpaceState) { s.ThinkState = "golang" }); err != nil {
		t.Fatalf("UpdateSpaceState failed: %v", err)
	}

	scaffoldChat(t, workspaceRoot, "golang", "chat-1")

	comparativeEval := "Combine agent-1 ServeMux with agent-2 JSON helper."
	finalReviewSummary := "Unified Go server combines ServeMux routing with clean JSON responses and passes all tests."

	// Stage 1: Manager evaluates candidate diffs on subgraph -> produces comparative evaluation
	// Stage 2: Worker executes YAML synthesize_prompt -> produces synthesized JSON patch
	// Stage 3: Manager writes final human-readable summary on main timeline before user review
	modelClient := &seTestModelClient{
		responses: []llm.StreamChunk{
			{Text: comparativeEval},
			{Text: `{"thought":"Merged patch","patches":[]}`},
			{Text: finalReviewSummary},
		},
	}
	turnExecutor := session.NewTurnExecutor(logger, modelClient, service)

	sessionManager := session.NewManager(logger, registry, svcManager, modelClient, nil, nil)
	chatSupervisor, err := sessionManager.GetSession(ctx, "golang", "chat-1")
	if err != nil {
		t.Fatalf("failed to get supervisor session: %v", err)
	}

	synthEngine := session.NewSynthesizeEngine(logger, registry, service, turnExecutor, chatSupervisor)

	flowID := uuid.NewV7()
	seedFlowNodeOnMain(t, service, chatSession, flowID)

	optionsNodeID := uuid.NewV7()
	flowName := fmt.Sprintf("flow-%s", flowID.String())
	flowAddress := &thinkgraph.ChatAddress{
		ChatID:    "chat-1",
		ProjectID: "golang",
		SubGraph: thinkgraph.SubGraph{
			Name:    flowName,
			GraphID: thinkgraph.GraphID{Prefix: thinkgraph.PrefixFlow, UUID: flowID},
		},
	}

	// Verify that while the user is being prompted to Accept/Reject:
	// 1. The subgraph does NOT prematurely contain EventCandidateResolution ("Proposal ACCEPTED").
	// 2. The main timeline ALREADY contains the Manager's final review analysis node.
	var ui *seTestUI
	ui = &seTestUI{
		reviewToReturn: true,
		reasonToReturn: "looks great, ship it",
		onReviewCandidate: func(branch string) {
			for _, n := range ui.streamedSubGraphs {
				if n.Type == workspace.EventCandidateResolution {
					t.Errorf("subgraph must not emit EventCandidateResolution before user responds to ReviewCandidate, got: %+v", n)
				}
			}
			midSubgraph, qErr := service.Graph().GetSubGraphTimeline(ctx, flowAddress, flowAddress.GraphID)
			if qErr != nil {
				t.Fatalf("GetSubGraphTimeline during ReviewCandidate failed: %v", qErr)
			}
			for _, n := range midSubgraph {
				if n.Type == workspace.EventCandidateResolution {
					t.Errorf("persisted subgraph must not contain EventCandidateResolution before user responds to ReviewCandidate, got: %+v", n)
				}
			}

			midMain, mErr := service.Graph().GetSubGraphTimeline(ctx, chatSession, chatSession.GraphID)
			if mErr != nil {
				t.Fatalf("GetSubGraphTimeline(main) during ReviewCandidate failed: %v", mErr)
			}
			foundReview := false
			for _, n := range midMain {
				if n.Type == thinkgraph.EventModel && n.Content == finalReviewSummary {
					foundReview = true
				}
			}
			if !foundReview {
				t.Errorf("expected Manager final review analysis %q to already be on main timeline when prompting user", finalReviewSummary)
			}
		},
	}

	workerCand1 := flows.Candidate{ID: fmt.Sprintf("chat-1-%s-agent-1", flowName), AgentID: "agent-1", Passed: true, Diff: "+ diff 1"}
	workerCand2 := flows.Candidate{ID: fmt.Sprintf("chat-1-%s-agent-2", flowName), AgentID: "agent-2", Passed: true, Diff: "+ diff 2"}
	scope := session.NewFlowScope(logger, service, nil, chatSession, flowAddress, flowID)
	scope.RegisterCandidate(workerCand1)
	scope.RegisterCandidate(workerCand2)

	synthSignal := &signals.SignalSynthesize{
		OriginalInstruction: "Evaluate branches: {{ range .Branches }}{{ .CandidateID }}{{ end }}",
		OriginalContext:     "Build fixtureapi HTTP service",
		Candidates:          []flows.Candidate{workerCand1, workerCand2},
		Flow:                scope,
		FlowAddress:         flowAddress,
		ParentNodeID:        optionsNodeID,
		AutoAccept:          false,
	}

	turnID := uuid.NewV7()
	signal, err := synthEngine.Execute(ctx, chatSession, turnID, uuid.NewV7(), synthSignal, ui, emitter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify Stage 1 (Manager eval), Stage 2 (Worker synth), and Stage 3 (Manager main review)
	if len(modelClient.capturedModels) != 3 {
		t.Fatalf("expected 3 LLM calls (Manager eval + Worker synth + Manager main review), got %d", len(modelClient.capturedModels))
	}
	if modelClient.capturedModels[0] != "gemini-3.8-flash" {
		t.Errorf("expected Stage 1 Manager evaluation to use 'gemini-3.8-flash', got %q", modelClient.capturedModels[0])
	}
	if modelClient.capturedModels[1] != "gemini-4-hydrogen" {
		t.Errorf("expected Stage 2 Worker synthesis to use 'gemini-4-hydrogen', got %q", modelClient.capturedModels[1])
	}
	if modelClient.capturedModels[2] != "gemini-3.8-flash" {
		t.Errorf("expected Stage 3 Manager main review to use 'gemini-3.8-flash', got %q", modelClient.capturedModels[2])
	}

	// Verify Stage 2 Worker received the YAML synthesize_prompt as Catalyst and Manager's evaluation in LongTermMemory
	if modelClient.capturedPrompts[1] != testSynthesizePromptYAML {
		t.Errorf("expected Stage 2 Worker Catalyst to equal YAML synthesize_prompt %q, got %q", testSynthesizePromptYAML, modelClient.capturedPrompts[1])
	}
	if len(modelClient.capturedLongTermMemory[1]) == 0 || !strings.Contains(modelClient.capturedLongTermMemory[1][0], comparativeEval) {
		t.Errorf("expected Stage 2 Worker LongTermMemory to contain Manager comparative evaluation %q, got %v", comparativeEval, modelClient.capturedLongTermMemory[1])
	}

	expectedCandidateID := fmt.Sprintf("chat-1-%s-synthesis", flowName)
	expectedBranch := fmt.Sprintf("candidate/%s", expectedCandidateID)

	if len(ui.reviewCalls) != 1 || ui.reviewCalls[0] != expectedBranch {
		t.Errorf("Expected ui.ReviewCandidate to be called once with %q, got %v", expectedBranch, ui.reviewCalls)
	}
	if engine.acceptedCandidate != expectedCandidateID {
		t.Errorf("Expected accepted candidate %q, got %q", expectedCandidateID, engine.acceptedCandidate)
	}

	if engine.rejected[workerCand1.ID] != "Superseded by synthesis" {
		t.Errorf("Expected superseded worker candidate %s to be rejected, got %q", workerCand1.ID, engine.rejected[workerCand1.ID])
	}
	if engine.rejected[workerCand2.ID] != "Superseded by synthesis" {
		t.Errorf("Expected superseded worker candidate %s to be rejected, got %q", workerCand2.ID, engine.rejected[workerCand2.ID])
	}

	if _, ok := signal.(*signals.SignalTerminal); !ok {
		t.Errorf("Expected SignalTerminal from synthesis, got %T", signal)
	}

	if len(ui.streamedSubGraphs) < 5 {
		t.Errorf("expected live subgraph nodes to stream to UI during synthesis, got %d streamed nodes", len(ui.streamedSubGraphs))
	}

	if err := service.SyncGraphContext(ctx, flowAddress); err != nil {
		t.Fatalf("SyncGraphContext failed: %v", err)
	}
	timeline, err := service.Graph().GetSubGraphTimeline(ctx, flowAddress, flowAddress.GraphID)
	if err != nil {
		t.Fatalf("GetSubGraphTimeline failed: %v", err)
	}

	if len(timeline) != 5 {
		t.Fatalf("Expected 5 synthesis nodes on subgraph, got %d", len(timeline))
	}

	if timeline[0].Type != thinkgraph.EventPrompt || timeline[0].Edges["parent_id"] != optionsNodeID.String() {
		t.Errorf("Expected timeline[0] to be refine prompt linked to optionsNodeID, got %+v", timeline[0])
	}
	if timeline[1].Type != thinkgraph.EventModel || timeline[1].Content != comparativeEval || timeline[1].Edges["parent_id"] != timeline[0].ID.String() {
		t.Errorf("Expected timeline[1] to hold Manager evaluation %q linked to timeline[0], got %+v", comparativeEval, timeline[1])
	}
	if timeline[2].Type != thinkgraph.EventPrompt || timeline[2].Content != testSynthesizePromptYAML || timeline[2].Edges["parent_id"] != timeline[1].ID.String() {
		t.Errorf("Expected timeline[2] to hold YAML synthesize_prompt %q linked to timeline[1], got %+v", testSynthesizePromptYAML, timeline[2])
	}
	if timeline[1].Content == timeline[2].Content {
		t.Errorf("Expected Manager evaluation node and Synthesis worker prompt node to have distinct content, but both were %q", timeline[1].Content)
	}
	if timeline[3].Type != thinkgraph.EventModel || timeline[3].Edges["parent_id"] != timeline[2].ID.String() {
		t.Errorf("Expected timeline[3] to be Worker synthesis model node linked to timeline[2], got %+v", timeline[3])
	}
	if timeline[4].Type != workspace.EventCandidateResolution || timeline[4].Edges["parent_id"] != timeline[3].ID.String() {
		t.Errorf("Expected timeline[4] to be final user CandidateResolution node linked to timeline[3], got %+v", timeline[4])
	}
	if !strings.Contains(timeline[4].Content, "ACCEPTED") || !strings.Contains(timeline[4].Content, "looks great, ship it") {
		t.Errorf("Expected timeline[4] resolution node to record user's acceptance reason, got %q", timeline[4].Content)
	}

	// Main timeline contains:
	// 0: FlowLifecycle (FlowStatusCompleted)
	// 1: Manager final review analysis EventModel (parent_id: optionsNodeID)
	// 2: CandidateResolution
	// 3: User decision EventPrompt (parent_id: mainTimeline[1].ID)
	mainTimeline, err := service.Graph().GetSubGraphTimeline(ctx, chatSession, chatSession.GraphID)
	if err != nil {
		t.Fatalf("GetSubGraphTimeline failed: %v", err)
	}
	if len(mainTimeline) != 4 {
		t.Fatalf("Expected 4 nodes on main timeline (FlowLifecycle + Manager Review + Resolution + User decision), got %d", len(mainTimeline))
	}
	if mainTimeline[0].Type != flows.EventFlowLifecycle {
		t.Errorf("Expected FlowLifecycle at mainTimeline[0], got %s", mainTimeline[0].Type)
	}
	var updatedFlowState flows.FlowExecutionState
	if err := json.Unmarshal(mainTimeline[0].MaterializedState, &updatedFlowState); err != nil {
		t.Fatalf("Failed to unmarshal FlowLifecycle state: %v", err)
	}
	if updatedFlowState.Status != flows.FlowStatusCompleted {
		t.Errorf("Expected FlowLifecycle status FlowStatusCompleted, got %v", updatedFlowState.Status)
	}
	if mainTimeline[1].Type != thinkgraph.EventModel || mainTimeline[1].Content != finalReviewSummary || mainTimeline[1].Edges["parent_id"] != optionsNodeID.String() {
		t.Errorf("Expected Manager review EventModel at mainTimeline[1] linked to optionsNodeID, got %+v", mainTimeline[1])
	}
	if mainTimeline[2].Type != workspace.EventCandidateResolution {
		t.Errorf("Expected CandidateResolution at mainTimeline[2], got %s", mainTimeline[2].Type)
	}
	if mainTimeline[3].Type != thinkgraph.EventPrompt || !strings.Contains(mainTimeline[3].Content, "accepted flow: looks great, ship it") || mainTimeline[3].Edges["parent_id"] != mainTimeline[1].ID.String() {
		t.Errorf("Expected user decision prompt at mainTimeline[3] linked to Manager review node, got %+v", mainTimeline[3])
	}
}

func TestSynthesizeEngine_MissingSynthesizePromptConfigFails(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	registry := config.NewRegistry()

	synthEngine := session.NewSynthesizeEngine(logger, registry, nil, nil, nil)
	_, err := synthEngine.Execute(ctx, &thinkgraph.ChatAddress{}, uuid.NewV7(), uuid.NewV7(), &signals.SignalSynthesize{}, &seTestUI{}, &seTestEmitter{})
	if err == nil {
		t.Fatalf("expected SynthesizeEngine.Execute to fail when fanout config is missing from registry")
	}
	if !strings.Contains(err.Error(), "fanout") {
		t.Errorf("expected error to reference missing fanout config, got: %v", err)
	}
}

func TestSynthesizeEngine_FailedVerification_AlwaysPromptsUserAndAllowsAccept(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	engine := &seTestChatEngine{}
	workspaceRoot := t.TempDir()

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("failed to init badger: %v", err)
	}
	defer graphDB.Close()

	emitter := &seTestEmitter{}
	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatVersioner { return engine }, graphDB, emitter)
	service := svcManager.GetService("golang")

	chatSession := &thinkgraph.ChatAddress{
		ChatID:    "chat-synth-fail",
		ProjectID: "golang",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}
	if err := service.InitializeChat(ctx, chatSession); err != nil {
		t.Fatalf("InitializeChat failed: %v", err)
	}

	registry := config.NewRegistry()
	registerFanoutFlowConfig(t, registry)
	mockSpace := &seTestThinkSpace{
		verifyErr: errors.New("go.mod requires go 1.27, found 1.22"),
	}
	registry.RegisterThinkState("golang", mockSpace)
	registry.RegisterConfig("golang", mockSpace.Config())
	if err := svcManager.UpdateSpaceState("golang", func(s *workspace.SpaceState) { s.ThinkState = "golang" }); err != nil {
		t.Fatalf("UpdateSpaceState failed: %v", err)
	}

	scaffoldChat(t, workspaceRoot, "golang", "chat-synth-fail")

	modelClient := &seTestModelClient{
		responses: []llm.StreamChunk{
			{Text: "Synthesize uuid implementation."},
			{Text: `{"thought":"Attempt 1 patch","patches":[]}`},
			{Text: `{"thought":"Attempt 2 patch","patches":[]}`},
			{Text: "Synthesized fix summary with verification warning."},
		},
	}
	turnExecutor := session.NewTurnExecutor(logger, modelClient, service)
	sessionManager := session.NewManager(logger, registry, svcManager, modelClient, nil, nil)
	chatSupervisor, err := sessionManager.GetSession(ctx, "golang", "chat-synth-fail")
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	synthEngine := session.NewSynthesizeEngine(logger, registry, service, turnExecutor, chatSupervisor)
	ui := &seTestUI{
		reviewToReturn: true,
		reasonToReturn: "Accepting despite go.mod version",
	}

	flowID := uuid.NewV7()
	seedFlowNodeOnMain(t, service, chatSession, flowID)

	flowName := fmt.Sprintf("flow-%s", flowID.String())
	flowAddress := &thinkgraph.ChatAddress{
		ChatID:    "chat-synth-fail",
		ProjectID: "golang",
		SubGraph: thinkgraph.SubGraph{
			Name:    flowName,
			GraphID: thinkgraph.GraphID{Prefix: thinkgraph.PrefixFlow, UUID: flowID},
		},
	}

	synthSignal := &signals.SignalSynthesize{
		OriginalInstruction: "Synthesize uuid fix",
		FlowAddress:         flowAddress,
		ParentNodeID:        uuid.NewV7(),
		AutoAccept:          true,
	}

	_, err = synthEngine.Execute(ctx, chatSession, uuid.NewV7(), uuid.NewV7(), synthSignal, ui, emitter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedCandidateID := fmt.Sprintf("chat-synth-fail-%s-synthesis", flowName)
	expectedBranch := fmt.Sprintf("candidate/%s", expectedCandidateID)

	if !engine.sandbox.delivered {
		t.Errorf("Expected synthesized candidate sandbox to be delivered for review even when verification fails")
	}
	if len(ui.reviewCalls) != 1 || ui.reviewCalls[0] != expectedBranch {
		t.Errorf("Expected ui.ReviewCandidate(%q) to be called on verification failure, got %v", expectedBranch, ui.reviewCalls)
	}
	if !engine.acceptCalled || engine.acceptedCandidate != expectedCandidateID {
		t.Errorf("Expected user acceptance to merge %q, got %q", expectedCandidateID, engine.acceptedCandidate)
	}

	mainTimeline, err := service.Graph().GetSubGraphTimeline(ctx, chatSession, chatSession.GraphID)
	if err != nil {
		t.Fatalf("GetSubGraphTimeline failed: %v", err)
	}
	var updatedFlowState flows.FlowExecutionState
	if err := json.Unmarshal(mainTimeline[0].MaterializedState, &updatedFlowState); err != nil {
		t.Fatalf("Failed to unmarshal FlowLifecycle state: %v", err)
	}
	if updatedFlowState.Status != flows.FlowStatusCompleted {
		t.Errorf("Expected FlowLifecycle to become FlowStatusCompleted after user accepted, got %v", updatedFlowState.Status)
	}
}

type alwaysFailPatcher struct{}

func (p *alwaysFailPatcher) Apply(ctx context.Context, sandbox workspace.CandidateSandbox, llmOutput string) error {
	return errors.New("invalid patch JSON: unrecoverable syntax")
}
func (p *alwaysFailPatcher) SystemInstructions() string { return "" }

func TestSynthesizeEngine_PatchFailureFallsBackToSurvivingWorkerCandidates(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	engine := &seTestChatEngine{}
	workspaceRoot := t.TempDir()

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("failed to init badger: %v", err)
	}
	defer graphDB.Close()

	emitter := &seTestEmitter{}
	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatVersioner { return engine }, graphDB, emitter)
	service := svcManager.GetService("golang")

	chatSession := &thinkgraph.ChatAddress{
		ChatID:    "chat-fallback",
		ProjectID: "golang",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}
	if err := service.InitializeChat(ctx, chatSession); err != nil {
		t.Fatalf("InitializeChat failed: %v", err)
	}

	registry := config.NewRegistry()
	registerFanoutFlowConfig(t, registry)
	mockSpace := &seTestThinkSpace{patcher: &alwaysFailPatcher{}}
	registry.RegisterThinkState("golang", mockSpace)
	registry.RegisterConfig("golang", mockSpace.Config())
	if err := svcManager.UpdateSpaceState("golang", func(s *workspace.SpaceState) { s.ThinkState = "golang" }); err != nil {
		t.Fatalf("UpdateSpaceState failed: %v", err)
	}

	scaffoldChat(t, workspaceRoot, "golang", "chat-fallback")

	modelClient := &seTestModelClient{
		responses: []llm.StreamChunk{
			{Text: "Manager synthesis instruction"},
			{Text: "not valid json attempt 1"},
			{Text: "not valid json attempt 2"},
		},
	}
	turnExecutor := session.NewTurnExecutor(logger, modelClient, service)
	sessionManager := session.NewManager(logger, registry, svcManager, modelClient, nil, nil)
	chatSupervisor, err := sessionManager.GetSession(ctx, "golang", "chat-fallback")
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	synthEngine := session.NewSynthesizeEngine(logger, registry, service, turnExecutor, chatSupervisor)
	ui := &seTestUI{reviewToReturn: true, reasonToReturn: "Picking surviving agent-1 branch"}

	flowID := uuid.NewV7()
	seedFlowNodeOnMain(t, service, chatSession, flowID)

	flowName := fmt.Sprintf("flow-%s", flowID.String())
	flowAddress := &thinkgraph.ChatAddress{
		ChatID:    "chat-fallback",
		ProjectID: "golang",
		SubGraph: thinkgraph.SubGraph{
			Name:    flowName,
			GraphID: thinkgraph.GraphID{Prefix: thinkgraph.PrefixFlow, UUID: flowID},
		},
	}

	workerCand1 := flows.Candidate{ID: fmt.Sprintf("chat-fallback-%s-agent-1", flowName), AgentID: "agent-1", Passed: true}
	scope := session.NewFlowScope(logger, service, nil, chatSession, flowAddress, flowID)
	scope.RegisterCandidate(workerCand1)

	synthSignal := &signals.SignalSynthesize{
		OriginalInstruction: "Synthesize",
		Candidates:          []flows.Candidate{workerCand1},
		Flow:                scope,
		FlowAddress:         flowAddress,
		ParentNodeID:        uuid.NewV7(),
		AutoAccept:          true,
	}

	_, err = synthEngine.Execute(ctx, chatSession, uuid.NewV7(), uuid.NewV7(), synthSignal, ui, emitter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ui.reviewCalls) != 1 || ui.reviewCalls[0] != workerCand1.BranchRef() {
		t.Fatalf("expected user to be prompted to review surviving worker candidate %q when synthesis failed to produce a patch, got %v", workerCand1.BranchRef(), ui.reviewCalls)
	}
	if !engine.acceptCalled || engine.acceptedCandidate != workerCand1.ID {
		t.Errorf("expected surviving worker candidate %q to be accepted by user, got %q", workerCand1.ID, engine.acceptedCandidate)
	}
}

func TestSynthesizeEngine_AutoAccept_SkipsManualReviewWhenVerified(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	engine := &seTestChatEngine{}
	workspaceRoot := t.TempDir()

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("failed to init badger: %v", err)
	}
	defer graphDB.Close()

	emitter := &seTestEmitter{}
	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatVersioner { return engine }, graphDB, emitter)
	service := svcManager.GetService("golang")

	chatSession := &thinkgraph.ChatAddress{
		ChatID:    "chat-auto",
		ProjectID: "golang",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}
	if err := service.InitializeChat(ctx, chatSession); err != nil {
		t.Fatalf("InitializeChat failed: %v", err)
	}

	registry := config.NewRegistry()
	registerFanoutFlowConfig(t, registry)
	mockSpace := &seTestThinkSpace{}
	registry.RegisterThinkState("golang", mockSpace)
	registry.RegisterConfig("golang", mockSpace.Config())
	if err := svcManager.UpdateSpaceState("golang", func(s *workspace.SpaceState) { s.ThinkState = "golang" }); err != nil {
		t.Fatalf("UpdateSpaceState failed: %v", err)
	}

	scaffoldChat(t, workspaceRoot, "golang", "chat-auto")

	modelClient := &seTestModelClient{
		responses: []llm.StreamChunk{
			{Text: "Manager synthesis architectural instruction"},
			{Text: `{"thought":"Merged patch","patches":[]}`},
			{Text: "Final synthesized solution summary on main timeline."},
		},
	}
	turnExecutor := session.NewTurnExecutor(logger, modelClient, service)
	sessionManager := session.NewManager(logger, registry, svcManager, modelClient, nil, nil)
	chatSupervisor, err := sessionManager.GetSession(ctx, "golang", "chat-auto")
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	synthEngine := session.NewSynthesizeEngine(logger, registry, service, turnExecutor, chatSupervisor)
	ui := &seTestUI{reviewToReturn: false}

	flowID := uuid.NewV7()
	seedFlowNodeOnMain(t, service, chatSession, flowID)

	flowName := fmt.Sprintf("flow-%s", flowID.String())
	flowAddress := &thinkgraph.ChatAddress{
		ChatID:    "chat-auto",
		ProjectID: "golang",
		SubGraph: thinkgraph.SubGraph{
			Name:    flowName,
			GraphID: thinkgraph.GraphID{Prefix: thinkgraph.PrefixFlow, UUID: flowID},
		},
	}

	synthSignal := &signals.SignalSynthesize{
		OriginalInstruction: "Do work",
		FlowAddress:         flowAddress,
		ParentNodeID:        uuid.NewV7(),
		AutoAccept:          true,
	}

	_, err = synthEngine.Execute(ctx, chatSession, uuid.NewV7(), uuid.NewV7(), synthSignal, ui, emitter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ui.reviewCalls) != 0 {
		t.Errorf("Expected ui.ReviewCandidate to be skipped when verified and AutoAccept is true, got %v", ui.reviewCalls)
	}

	expectedCandidateID := fmt.Sprintf("chat-auto-%s-synthesis", flowName)
	if engine.acceptedCandidate != expectedCandidateID {
		t.Errorf("Expected auto-accepted candidate %q, got %q", expectedCandidateID, engine.acceptedCandidate)
	}

	mainTimeline, err := service.Graph().GetSubGraphTimeline(ctx, chatSession, chatSession.GraphID)
	if err != nil {
		t.Fatalf("GetSubGraphTimeline failed: %v", err)
	}
	if len(mainTimeline) != 3 {
		t.Fatalf("Expected 3 nodes (FlowLifecycle + Manager Review + Resolution) on main timeline when AutoAccept is true, got %d", len(mainTimeline))
	}
	if mainTimeline[1].Type != thinkgraph.EventModel {
		t.Errorf("Expected Manager review EventModel at mainTimeline[1], got %s", mainTimeline[1].Type)
	}
	if mainTimeline[2].Type != workspace.EventCandidateResolution {
		t.Errorf("Expected CandidateResolution at mainTimeline[2], got %s", mainTimeline[2].Type)
	}
}
