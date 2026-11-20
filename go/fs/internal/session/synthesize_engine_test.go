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

const testSynthesizePromptYAML = "Find the best architectural patterns across candidate branches and merge them."

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
  synthesis_review_prompt: "Review synthesized proposal {{ .CandidateID }}{{ if .Reasoning }}\nRationale: {{ .Reasoning }}{{ end }}"
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
	capturedWorkbenches    [][]cognition.WorkbenchItem
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
	client.capturedWorkbenches = append(client.capturedWorkbenches, append([]cognition.WorkbenchItem(nil), bundle.Workbench...))
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
	spawnCount        int
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
	m.spawnCount++
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
		Name: "golang",
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
func (m *seTestThinkSpace) Verify(ctx context.Context, runner workspace.CommandRunner) error {
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
	contextReviews    []session.TurnContext
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
	u.contextReviews = append(u.contextReviews, ctxData)
	return session.ContextApprovalDecision{Approved: true}
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

func TestSynthesizeEngine_SinglePassASTSynthesisAndMainReview(t *testing.T) {
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
	writeWorkbenchFile(t, service, "golang", "src/existing.go", "package main\nfunc existing() {}\n")

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

	mergeThought := "Adopted Server struct from agent-2 and resilient header check from agent-1."
	finalReviewSummary := "Unified Go server combines ServeMux routing with clean JSON responses and passes all tests."

	// Call 0: Worker executes YAML synthesize_prompt -> produces merge thought + JSON patch
	// Call 1: Manager writes final human-readable summary on main timeline before user review
	modelClient := &seTestModelClient{
		responses: []llm.StreamChunk{
			{Text: fmt.Sprintf(`{"thought":%q,"patches":[]}`, mergeThought)},
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

	workerCand1 := flows.Candidate{
		ID:      fmt.Sprintf("chat-1-%s-agent-1", flowName),
		AgentID: "agent-1",
		Passed:  true,
		Diff:    "+ diff 1",
		Files: map[string]string{
			"src/main.go": "package main\n\nfunc newMux() {}\n",
		},
	}
	workerCand2 := flows.Candidate{
		ID:      fmt.Sprintf("chat-1-%s-agent-2", flowName),
		AgentID: "agent-2",
		Passed:  true,
		Diff:    "+ diff 2",
		Files: map[string]string{
			"src/internal/server/server.go": "package server\n\ntype Server struct{}\n",
		},
	}
	scope := session.NewFlowScope(logger, service, nil, chatSession, flowAddress, flowID)
	scope.RegisterCandidate(workerCand1)
	scope.RegisterCandidate(workerCand2)

	synthSignal := &signals.SignalSynthesize{
		OriginalInstruction: "Evaluate branches",
		OriginalContext:     "Build fixtureapi HTTP service",
		Candidates:          []flows.Candidate{workerCand1, workerCand2},
		Flow:                scope,
		FlowAddress:         flowAddress,
		ParentNodeID:        optionsNodeID,
		AutoAccept:          false,
		TargetFiles:         []string{"src/existing.go", "src/main.go"},
	}

	turnID := uuid.NewV7()
	signal, err := synthEngine.Execute(ctx, chatSession, turnID, uuid.NewV7(), synthSignal, ui, emitter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify 2 LLM calls: Worker synthesis + Manager main review
	if len(modelClient.capturedModels) != 2 {
		t.Fatalf("expected 2 LLM calls (Worker synth + Manager main review), got %d", len(modelClient.capturedModels))
	}
	if modelClient.capturedModels[0] != "gemini-4-hydrogen" {
		t.Errorf("expected Stage 1 Worker synthesis to use 'gemini-4-hydrogen', got %q", modelClient.capturedModels[0])
	}
	if modelClient.capturedModels[1] != "gemini-3.8-flash" {
		t.Errorf("expected Stage 2 Manager main review to use 'gemini-3.8-flash', got %q", modelClient.capturedModels[1])
	}

	// Verify Synthesis Worker received AST-projected candidate symbols/code and pre-existing base workbench file
	if modelClient.capturedPrompts[0] != testSynthesizePromptYAML {
		t.Errorf("expected Worker Catalyst to equal YAML synthesize_prompt %q, got %q", testSynthesizePromptYAML, modelClient.capturedPrompts[0])
	}
	if len(modelClient.capturedLongTermMemory[0]) == 0 {
		t.Fatalf("expected Worker LongTermMemory to be populated")
	}
	workerDigest := modelClient.capturedLongTermMemory[0][0]
	for _, want := range []string{"Build fixtureapi HTTP service", "`func newMux()`", "`struct Server`"} {
		if !strings.Contains(workerDigest, want) {
			t.Errorf("expected Worker LongTermMemory to contain %q, got:\n%s", want, workerDigest)
		}
	}
	if len(modelClient.capturedWorkbenches[0]) != 1 || modelClient.capturedWorkbenches[0][0].Path != "src/existing.go" {
		t.Errorf("expected only pre-existing base file 'src/existing.go' in Synthesis Worker workbench, got %+v", modelClient.capturedWorkbenches[0])
	}

	// Verify Manager main review received the Synthesis Worker's extracted merge rationale
	if !strings.Contains(modelClient.capturedPrompts[1], mergeThought) {
		t.Errorf("expected Manager review prompt to include extracted merge rationale %q, got:\n%s", mergeThought, modelClient.capturedPrompts[1])
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

	if err := service.SyncGraphContext(ctx, flowAddress); err != nil {
		t.Fatalf("SyncGraphContext failed: %v", err)
	}
	timeline, err := service.Graph().GetSubGraphTimeline(ctx, flowAddress, flowAddress.GraphID)
	if err != nil {
		t.Fatalf("GetSubGraphTimeline failed: %v", err)
	}

	if len(timeline) != 3 {
		t.Fatalf("Expected 3 synthesis nodes on subgraph (Prompt + Model + Resolution), got %d", len(timeline))
	}
	if timeline[0].Type != thinkgraph.EventPrompt || timeline[0].Content != testSynthesizePromptYAML || timeline[0].Edges["parent_id"] != optionsNodeID.String() {
		t.Errorf("Expected timeline[0] to be synthesize prompt linked directly to optionsNodeID, got %+v", timeline[0])
	}
	if timeline[1].Type != thinkgraph.EventModel || timeline[1].Edges["parent_id"] != timeline[0].ID.String() {
		t.Errorf("Expected timeline[1] to be Worker synthesis model node linked to timeline[0], got %+v", timeline[1])
	}
	if timeline[2].Type != workspace.EventCandidateResolution || timeline[2].Edges["parent_id"] != timeline[1].ID.String() {
		t.Errorf("Expected timeline[2] to be final user CandidateResolution node linked to timeline[1], got %+v", timeline[2])
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

func (p *alwaysFailPatcher) Apply(ctx context.Context, fs workspace.FileWorkspace, llmOutput string) error {
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
			{Text: `{"thought":"Merged best patterns","patches":[]}`},
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
