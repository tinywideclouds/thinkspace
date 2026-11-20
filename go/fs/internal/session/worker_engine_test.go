package session_test

import (
	"context"
	"encoding/json"
	"errors"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

type weMockModelClient struct {
	responses []llm.StreamChunk
}

func (client *weMockModelClient) GenerateContentStream(
	ctx context.Context,
	model string,
	bundle cognition.ThinkBundle,
	registry *tools.Registry,
	config spaces.ThinkStateConfig,
) iter.Seq2[llm.StreamChunk, error] {
	return func(yield func(llm.StreamChunk, error) bool) {
		for _, resp := range client.responses {
			if !yield(resp, nil) {
				return
			}
		}
	}
}

type weMockChatEngine struct {
	mu           sync.Mutex
	sandbox      *weMockSandbox
	spawnCount   int
	acceptCalled bool
	rejectedIDs  []string
	rejectReason string
}

func (m *weMockChatEngine) InitChat(ctx context.Context, chatID string) error { return nil }
func (m *weMockChatEngine) Snapshot(ctx context.Context, chatID string, msg string) (string, error) {
	return "", nil
}
func (m *weMockChatEngine) PreviewCandidate(ctx context.Context, chatID string, candidateID string) error {
	return nil
}
func (m *weMockChatEngine) ReadCandidateDiff(ctx context.Context, chatID string, candidateID string) (string, error) {
	return "+ mock diff", nil
}
func (m *weMockChatEngine) Accept(ctx context.Context, chatID string, candidateID string, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.acceptCalled = true
	return nil
}
func (m *weMockChatEngine) Reject(ctx context.Context, chatID string, candidateID string, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rejectedIDs = append(m.rejectedIDs, candidateID)
	m.rejectReason = reason
	return nil
}
func (m *weMockChatEngine) SpawnCandidateSandbox(ctx context.Context, chatID string, candidateID string) (workspace.CandidateSandbox, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.spawnCount++
	m.sandbox = &weMockSandbox{candidateID: candidateID}
	return m.sandbox, nil
}

type weMockSandbox struct {
	candidateID string
	delivered   bool
}

func (m *weMockSandbox) ResolvePath(path string) string                                { return path }
func (m *weMockSandbox) WriteFile(ctx context.Context, path string, data []byte) error { return nil }
func (m *weMockSandbox) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if path == "does/not/exist.go" {
		return nil, errors.New("file not found")
	}
	return []byte("mock file content"), nil
}
func (m *weMockSandbox) ExecuteCommand(ctx context.Context, cmd string, args ...string) (string, error) {
	return "", nil
}
func (m *weMockSandbox) ApplyDraft(ctx context.Context, message string) error { return nil }
func (m *weMockSandbox) DeliverForReview(ctx context.Context) error           { m.delivered = true; return nil }
func (m *weMockSandbox) TearDown(ctx context.Context) error                   { return nil }

type weMockPatcher struct{}

func (p *weMockPatcher) Apply(ctx context.Context, fs workspace.FileWorkspace, llmOutput string) error {
	return nil
}
func (p *weMockPatcher) SystemInstructions() string { return "" }

type weMockThinkSpace struct {
	errsToReturn []error
	verifyCalls  int
}

func (m *weMockThinkSpace) Initialize(ctx context.Context, directory string) error { return nil }
func (m *weMockThinkSpace) Config() spaces.ThinkStateConfig                        { return spaces.ThinkStateConfig{} }
func (m *weMockThinkSpace) Patcher() workspace.Patcher                             { return &weMockPatcher{} }
func (m *weMockThinkSpace) Verifier() spaces.Verifier                              { return m }
func (m *weMockThinkSpace) Verify(ctx context.Context, runner workspace.CommandRunner) error {
	if m.verifyCalls < len(m.errsToReturn) {
		err := m.errsToReturn[m.verifyCalls]
		m.verifyCalls++
		return err
	}
	m.verifyCalls++
	return nil
}
func (m *weMockThinkSpace) Mapbook() spaces.Mapbook { return m }
func (m *weMockThinkSpace) GenerateLayers(ctx context.Context, root string) (map[spaces.MapbookLayer]string, error) {
	return map[spaces.MapbookLayer]string{spaces.LayerStructural: "Mock Tree"}, nil
}
func (m *weMockThinkSpace) EnrichBundle(ctx context.Context, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus, workspaceRoot string) error {
	return nil
}

type weMockEmitter struct {
	events []telemetry.Event
}

func (m *weMockEmitter) Emit(channel string, event telemetry.Event) {
	m.events = append(m.events, event)
}

type weMockUI struct {
	strategyToReturn   flows.FlowStrategy
	autoAcceptToReturn bool
	reviewToReturn     bool
	reasonToReturn     string
	reviewedBranch     string
	transitionToReturn bool
	capturedReasoning  string
	streamedMainNodes  []thinkgraph.Node
}

func (u *weMockUI) OnTextChunk(chunk workspace.NodeChunk) {}
func (u *weMockUI) StreamGraphEvent(address *thinkgraph.ChatAddress, node thinkgraph.Node) {
	if address != nil && address.Name == "main" {
		u.streamedMainNodes = append(u.streamedMainNodes, node)
	}
}
func (u *weMockUI) ReviewContext(ctxData session.TurnContext) session.ContextApprovalDecision {
	return session.ContextApprovalDecision{Approved: u.reviewToReturn}
}
func (u *weMockUI) ChooseNextStep() (flows.FlowStrategy, bool) {
	return u.strategyToReturn, u.autoAcceptToReturn
}
func (u *weMockUI) ReviewCandidate(branch string) (bool, string) {
	u.reviewedBranch = branch
	return u.reviewToReturn, u.reasonToReturn
}
func (u *weMockUI) ConfirmTransition(targetThinkState string, reasoning string) bool {
	u.capturedReasoning = reasoning
	return u.transitionToReturn
}
func (u *weMockUI) GetChunkChannel() chan<- workspace.NodeChunk { return nil }

func writeWorkbenchFile(t *testing.T, svc *workspace.Service, spaceID, relPath, content string) {
	t.Helper()
	full := filepath.Join(svc.WorkbenchDir(spaceID), filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
}

func TestWorkerEngine_FanOutIntegration_TopologyAndEdges(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	engine := &weMockChatEngine{}
	workspaceRoot := t.TempDir()

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("failed to init badger: %v", err)
	}
	defer graphDB.Close()

	graphStore := thinkgraph.NewGraphStore(graphDB, nil)
	ledger := thinkgraph.NewFileLedger(1000, workspaceRoot)
	dispatcher := thinkgraph.NewDispatcher(graphStore, ledger)

	emitter := &weMockEmitter{}
	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatVersioner { return engine }, graphDB, emitter)
	service := svcManager.GetService("golang")
	writeWorkbenchFile(t, service, "golang", "src/main.go", "package main\n")

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
	mockSpace := &weMockThinkSpace{}
	registry.RegisterThinkState("golang", mockSpace)
	registry.RegisterConfig("golang", mockSpace.Config())
	if err := svcManager.UpdateSpaceState("golang", func(s *workspace.SpaceState) { s.ThinkState = "golang" }); err != nil {
		t.Fatalf("UpdateSpaceState failed: %v", err)
	}

	scaffoldChat(t, workspaceRoot, "golang", "chat-1")

	modelClient := &weMockModelClient{
		responses: []llm.StreamChunk{{Text: "Sub-agent output."}},
	}
	turnExecutor := session.NewTurnExecutor(logger, modelClient, service)

	flowCfg := flows.FanOutConfig{MaxSteps: 10, RetryPrompt: "Trace: {{.ErrorTrace}}"}
	fanOutFlow := flows.NewFanOutFlow(logger, flowCfg)

	sessionManager := session.NewManager(logger, registry, svcManager, modelClient, fanOutFlow, nil)
	chatSupervisor, err := sessionManager.GetSession(ctx, "golang", "chat-1")
	if err != nil {
		t.Fatalf("failed to get supervisor session: %v", err)
	}

	workerEngine := session.NewWorkerEngine(logger, chatSupervisor, registry, service, turnExecutor, dispatcher, fanOutFlow)
	ui := &weMockUI{strategyToReturn: flows.StrategyRefine, autoAcceptToReturn: true, reviewToReturn: true}

	tasks := []flows.SubAgentTask{
		{
			AgentID:       "agent-1",
			ContextDigest: "Basic server built",
			Instruction:   "Add auth",
			TargetFiles:   []string{"src/main.go"},
		},
	}

	dispatchSignal := &signals.SignalDispatchFlow{
		Intention: "Adding Bearer token authentication middleware to protected endpoints",
		Tasks:     tasks,
	}

	turnID := uuid.NewV7()
	catalystID := uuid.NewV7()

	signal, err := workerEngine.Execute(ctx, chatSession, turnID, catalystID, dispatchSignal, ui, emitter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	synthSig, ok := signal.(*signals.SignalSynthesize)
	if !ok {
		t.Fatalf("Expected SignalSynthesize based on StrategyRefine yield, got %T", signal)
	}

	if synthSig.Flow == nil {
		t.Fatalf("Expected SignalSynthesize to carry active FlowHandle for ownership handoff")
	}

	if len(synthSig.TargetFiles) != 1 || synthSig.TargetFiles[0] != "src/main.go" {
		t.Errorf("Expected TargetFiles ['src/main.go'] to propagate into SignalSynthesize, got %v", synthSig.TargetFiles)
	}

	if len(engine.rejectedIDs) != 0 {
		t.Errorf("Expected WorkerEngine not to reject candidates on Refine handoff, got rejected: %v", engine.rejectedIDs)
	}

	if !synthSig.AutoAccept {
		t.Errorf("Expected AutoAccept=true to propagate into SignalSynthesize")
	}

	if synthSig.FlowAddress == nil {
		t.Errorf("Expected synthesise signal to carry active FlowAddress for cross-boundary routing")
	}

	mainTimeline, err := service.Graph().GetSubGraphTimeline(ctx, chatSession, chatSession.GraphID)
	if err != nil {
		t.Fatalf("GetSubGraphTimeline failed: %v", err)
	}
	var flowCard *thinkgraph.Node
	var optionsNode *thinkgraph.Node
	for _, n := range mainTimeline {
		if n.Type == flows.EventFlowLifecycle {
			nodeCopy := n
			flowCard = &nodeCopy
		} else if n.Type == thinkgraph.EventSystem && strings.Contains(n.Content, "Awaiting delegation strategy") {
			nodeCopy := n
			optionsNode = &nodeCopy
		}
	}

	if flowCard == nil {
		t.Fatalf("Expected Flow Card on main timeline")
	}
	if flowCard.Edges["parent_id"] != turnID.String() {
		t.Errorf("Expected Flow Card to point parent_id to Manager Node (turnID), got %v", flowCard.Edges)
	}
	if flowCard.Content != "Adding Bearer token authentication middleware to protected endpoints" {
		t.Errorf("Expected Flow Card content to display Manager intention, got %q", flowCard.Content)
	}
	var flowData flows.FlowData
	if err := json.Unmarshal(flowCard.Data, &flowData); err != nil {
		t.Fatalf("Failed to unmarshal FlowData: %v", err)
	}
	if flowData.TaskPrompt != "Adding Bearer token authentication middleware to protected endpoints" {
		t.Errorf("Expected FlowData.TaskPrompt to carry Manager intention for Flow Card header, got %q", flowData.TaskPrompt)
	}

	var flowExecState flows.FlowExecutionState
	if err := json.Unmarshal(flowCard.MaterializedState, &flowExecState); err != nil {
		t.Fatalf("Failed to unmarshal flow card state: %v", err)
	}
	if flowExecState.Status != flows.FlowStatusRunning {
		t.Errorf("Expected Flow Card on main timeline to remain FlowStatusRunning until synthesis completes, got %v", flowExecState.Status)
	}

	if optionsNode == nil {
		t.Fatalf("Expected Flow Options Node on main timeline")
	}

	if synthSig.ParentNodeID != optionsNode.ID {
		t.Errorf("Expected yielded signal ParentNodeID to match the Flow Options node ID")
	}

	subGraphAddr := synthSig.FlowAddress
	if err := service.SyncGraphContext(ctx, subGraphAddr); err != nil {
		t.Fatalf("SyncGraphContext failed: %v", err)
	}
	flowTimeline, err := service.Graph().GetSubGraphTimeline(ctx, subGraphAddr, subGraphAddr.GraphID)
	if err != nil {
		t.Fatalf("GetSubGraphTimeline failed: %v", err)
	}

	var taskNode *thinkgraph.Node
	var joinNode *thinkgraph.Node
	for _, n := range flowTimeline {
		if n.Metadata["agent_id"] == "agent-1" && n.Type == thinkgraph.EventPrompt {
			nodeCopy := n
			taskNode = &nodeCopy
		} else if n.Type == thinkgraph.EventSystem && strings.Contains(n.Content, "Generated") {
			nodeCopy := n
			joinNode = &nodeCopy
		}
	}

	if taskNode == nil {
		t.Fatalf("Expected Task Node in subgraph")
	}
	if taskNode.Edges["parent_id"] != turnID.String() {
		t.Errorf("Expected Subgraph Task to point cross-boundary parent_id to Manager Node (turnID), got %v", taskNode.Edges)
	}

	if joinNode == nil {
		t.Fatalf("Expected Join Node in subgraph")
	}

	if optionsNode.Edges["parent_id"] != joinNode.ID.String() {
		t.Errorf("Expected Flow Options node (Main) to point cross-boundary parent_id to Join Node (Subgraph), got %v", optionsNode.Edges)
	}
}

func TestWorkerEngine_SkipStrategy_CleansUpAllCandidateBranches(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	engine := &weMockChatEngine{}
	workspaceRoot := t.TempDir()

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("failed to init badger: %v", err)
	}
	defer graphDB.Close()

	graphStore := thinkgraph.NewGraphStore(graphDB, nil)
	ledger := thinkgraph.NewFileLedger(1000, workspaceRoot)
	dispatcher := thinkgraph.NewDispatcher(graphStore, ledger)

	emitter := &weMockEmitter{}
	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatVersioner { return engine }, graphDB, emitter)
	service := svcManager.GetService("golang")

	chatSession := &thinkgraph.ChatAddress{
		ChatID:    "chat-skip",
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
	mockSpace := &weMockThinkSpace{}
	registry.RegisterThinkState("golang", mockSpace)
	registry.RegisterConfig("golang", mockSpace.Config())
	if err := svcManager.UpdateSpaceState("golang", func(s *workspace.SpaceState) { s.ThinkState = "golang" }); err != nil {
		t.Fatalf("UpdateSpaceState failed: %v", err)
	}

	scaffoldChat(t, workspaceRoot, "golang", "chat-skip")

	modelClient := &weMockModelClient{
		responses: []llm.StreamChunk{{Text: "Sub-agent output."}},
	}
	turnExecutor := session.NewTurnExecutor(logger, modelClient, service)
	fanOutFlow := flows.NewFanOutFlow(logger, flows.FanOutConfig{MaxSteps: 10})

	sessionManager := session.NewManager(logger, registry, svcManager, modelClient, fanOutFlow, nil)
	chatSupervisor, err := sessionManager.GetSession(ctx, "golang", "chat-skip")
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	workerEngine := session.NewWorkerEngine(logger, chatSupervisor, registry, service, turnExecutor, dispatcher, fanOutFlow)
	ui := &weMockUI{strategyToReturn: flows.StrategySkip}

	dispatchSignal := &signals.SignalDispatchFlow{
		Tasks: []flows.SubAgentTask{
			{AgentID: "agent-1", Instruction: "Draft approach A"},
		},
	}

	finalSig, err := workerEngine.Execute(ctx, chatSession, uuid.NewV7(), uuid.NewV7(), dispatchSignal, ui, emitter)
	if err != nil {
		t.Fatalf("unexpected error on StrategySkip: %v", err)
	}
	if !finalSig.IsTerminal() {
		t.Fatalf("expected SignalTerminal on StrategySkip, got %T", finalSig)
	}

	if len(engine.rejectedIDs) != 1 {
		t.Fatalf("expected FlowScope.Finalize to reject 1 skipped candidate branch, got %d", len(engine.rejectedIDs))
	}
	if engine.rejectReason != "Skipped by user" {
		t.Errorf("expected reject reason 'Skipped by user', got %q", engine.rejectReason)
	}
}

func TestWorkerEngine_FailedVerification_ReportsToMainAndAllowsAccept(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	engine := &weMockChatEngine{}
	workspaceRoot := t.TempDir()

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("failed to init badger: %v", err)
	}
	defer graphDB.Close()

	graphStore := thinkgraph.NewGraphStore(graphDB, nil)
	ledger := thinkgraph.NewFileLedger(1000, workspaceRoot)
	dispatcher := thinkgraph.NewDispatcher(graphStore, ledger)

	emitter := &weMockEmitter{}
	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatVersioner { return engine }, graphDB, emitter)
	service := svcManager.GetService("golang")

	chatSession := &thinkgraph.ChatAddress{
		ChatID:    "chat-fail-accept",
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
	mockSpace := &weMockThinkSpace{
		errsToReturn: []error{
			errors.New("go.mod requires go 1.27, found 1.22"),
			errors.New("go.mod requires go 1.27, found 1.22"),
		},
	}
	registry.RegisterThinkState("golang", mockSpace)
	registry.RegisterConfig("golang", mockSpace.Config())
	if err := svcManager.UpdateSpaceState("golang", func(s *workspace.SpaceState) { s.ThinkState = "golang" }); err != nil {
		t.Fatalf("UpdateSpaceState failed: %v", err)
	}

	scaffoldChat(t, workspaceRoot, "golang", "chat-fail-accept")

	modelClient := &weMockModelClient{
		responses: []llm.StreamChunk{
			{Text: "Attempt 1 patch"},
			{Text: "Attempt 2 patch"},
		},
	}
	turnExecutor := session.NewTurnExecutor(logger, modelClient, service)
	fanOutFlow := flows.NewFanOutFlow(logger, flows.FanOutConfig{MaxSteps: 10})

	sessionManager := session.NewManager(logger, registry, svcManager, modelClient, fanOutFlow, nil)
	chatSupervisor, err := sessionManager.GetSession(ctx, "golang", "chat-fail-accept")
	if err != nil {
		t.Fatalf("failed to get supervisor session: %v", err)
	}

	workerEngine := session.NewWorkerEngine(logger, chatSupervisor, registry, service, turnExecutor, dispatcher, fanOutFlow)
	ui := &weMockUI{
		strategyToReturn: flows.StrategyManual,
		reviewToReturn:   true,
		reasonToReturn:   "Accepting despite go.mod version",
	}

	dispatchSignal := &signals.SignalDispatchFlow{
		Tasks: []flows.SubAgentTask{
			{
				AgentID:     "agent-1",
				Instruction: "Use native uuid",
			},
		},
	}

	finalSig, err := workerEngine.Execute(ctx, chatSession, uuid.NewV7(), uuid.NewV7(), dispatchSignal, ui, emitter)
	if err != nil {
		t.Fatalf("expected failed verification flow to allow manual review/accept without returning fatal error, got: %v", err)
	}
	if !finalSig.IsTerminal() {
		t.Errorf("expected terminal signal after manual review accept, got %T", finalSig)
	}

	sawFailedCard := false
	for _, n := range ui.streamedMainNodes {
		if n.Type == flows.EventFlowLifecycle && len(n.MaterializedState) > 0 {
			var st flows.FlowExecutionState
			if json.Unmarshal(n.MaterializedState, &st) == nil && st.Status == flows.FlowStatusFailed {
				if strings.Contains(st.Error, "go.mod requires go 1.27") {
					sawFailedCard = true
				}
			}
		}
	}
	if !sawFailedCard {
		t.Errorf("expected main timeline Flow Card to stream FlowStatusFailed with verification error trace")
	}

	if !engine.acceptCalled {
		t.Errorf("expected user to be able to accept the failed candidate branch")
	}
}

func TestWorkerEngine_FailFast_HallucinatedFile_BeforeSandboxCreation(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	engine := &weMockChatEngine{}
	workspaceRoot := t.TempDir()

	ledger := thinkgraph.NewFileLedger(1000, workspaceRoot)
	dispatcher := thinkgraph.NewDispatcher(nil, ledger)
	emitter := &weMockEmitter{}
	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatVersioner { return engine }, nil, emitter)
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
	mockSpace := &weMockThinkSpace{}
	registry.RegisterThinkState("golang", mockSpace)
	registry.RegisterConfig("golang", mockSpace.Config())
	if err := svcManager.UpdateSpaceState("golang", func(s *workspace.SpaceState) { s.ThinkState = "golang" }); err != nil {
		t.Fatalf("UpdateSpaceState failed: %v", err)
	}

	scaffoldChat(t, workspaceRoot, "golang", "chat-1")

	turnExecutor := session.NewTurnExecutor(logger, &weMockModelClient{}, service)
	fanOutFlow := flows.NewFanOutFlow(logger, flows.FanOutConfig{MaxSteps: 10})

	sessionManager := session.NewManager(logger, registry, svcManager, &weMockModelClient{}, fanOutFlow, nil)
	chatSupervisor, err := sessionManager.GetSession(ctx, "golang", "chat-1")
	if err != nil {
		t.Fatalf("failed to get supervisor session: %v", err)
	}

	workerEngine := session.NewWorkerEngine(logger, chatSupervisor, registry, service, turnExecutor, dispatcher, fanOutFlow)

	tasks := []flows.SubAgentTask{
		{
			Instruction: "Edit hallucinated file",
			TargetFiles: []string{"does/not/exist.go"},
		},
	}
	dispatchSignal := &signals.SignalDispatchFlow{Tasks: tasks}

	_, err = workerEngine.Execute(ctx, chatSession, uuid.NewV7(), uuid.NewV7(), dispatchSignal, &weMockUI{}, emitter)

	if err == nil {
		t.Fatalf("expected flow to fail-fast on missing file, but it succeeded")
	}
	if !strings.Contains(err.Error(), "fatal system errors") {
		t.Errorf("expected fatal system error string, got: %v", err)
	}
	if engine.spawnCount != 0 {
		t.Errorf("expected zero sandboxes to be spawned on pre-branch fail-fast, got %d", engine.spawnCount)
	}
}

type workerReviewCapturingUI struct {
	weMockUI
	reviewCalls       []session.TurnContext
	retainedFilePaths []string
}

func (u *workerReviewCapturingUI) ReviewContext(ctxData session.TurnContext) session.ContextApprovalDecision {
	u.reviewCalls = append(u.reviewCalls, ctxData)
	return session.ContextApprovalDecision{
		Approved:          u.reviewToReturn,
		RetainedFilePaths: u.retainedFilePaths,
	}
}

type bundleRecordingModelClient struct {
	mu              sync.Mutex
	capturedBundles []cognition.ThinkBundle
}

func (c *bundleRecordingModelClient) GenerateContentStream(
	ctx context.Context,
	model string,
	bundle cognition.ThinkBundle,
	registry *tools.Registry,
	config spaces.ThinkStateConfig,
) iter.Seq2[llm.StreamChunk, error] {
	c.mu.Lock()
	c.capturedBundles = append(c.capturedBundles, bundle)
	c.mu.Unlock()
	return func(yield func(llm.StreamChunk, error) bool) {
		yield(llm.StreamChunk{Text: "Sub-agent output."}, nil)
	}
}

func TestWorkerEngine_BundledWorkerContextReviewAndFilePruning(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
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
	writeWorkbenchFile(t, service, "golang", "src/keep.go", "package main\nfunc keep() {}\n")
	writeWorkbenchFile(t, service, "golang", "src/drop.go", "package main\nfunc drop() {}\n")

	chatSession := &thinkgraph.ChatAddress{
		ChatID:    "chat-worker-review",
		ProjectID: "golang",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}
	if err := service.InitializeChat(ctx, chatSession); err != nil {
		t.Fatalf("InitializeChat failed: %v", err)
	}

	if err := svcManager.UpdateSpaceSettings("golang", func(s *workspace.SpaceSettings) {
		s.ReviewPolicy.Active = false
		s.WorkerReviewPolicy.Active = true
		s.WorkerReviewPolicy.MinTokens = 1
	}); err != nil {
		t.Fatalf("UpdateSpaceSettings failed: %v", err)
	}

	registry := config.NewRegistry()
	mockSpace := &weMockThinkSpace{}
	registry.RegisterThinkState("golang", mockSpace)
	registry.RegisterConfig("golang", mockSpace.Config())
	if err := svcManager.UpdateSpaceState("golang", func(s *workspace.SpaceState) { s.ThinkState = "golang" }); err != nil {
		t.Fatalf("UpdateSpaceState failed: %v", err)
	}
	scaffoldChat(t, workspaceRoot, "golang", "chat-worker-review")

	recordingClient := &bundleRecordingModelClient{}
	turnExecutor := session.NewTurnExecutor(logger, recordingClient, service)
	fanOutFlow := flows.NewFanOutFlow(logger, flows.FanOutConfig{MaxSteps: 10})

	sessionManager := session.NewManager(logger, registry, svcManager, recordingClient, fanOutFlow, nil)
	chatSupervisor, err := sessionManager.GetSession(ctx, "golang", "chat-worker-review")
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	workerEngine := session.NewWorkerEngine(logger, chatSupervisor, registry, service, turnExecutor, service.Dispatcher(), fanOutFlow)

	ui := &workerReviewCapturingUI{
		weMockUI:          weMockUI{strategyToReturn: flows.StrategySkip, reviewToReturn: true},
		retainedFilePaths: []string{"src/keep.go"},
	}

	dispatchSignal := &signals.SignalDispatchFlow{
		Intention: "Explore two routing designs",
		Tasks: []flows.SubAgentTask{
			{AgentID: "agent-1", ContextDigest: "Project overview", Instruction: "Approach 1", TargetFiles: []string{"src/keep.go", "src/drop.go"}},
			{AgentID: "agent-2", ContextDigest: "Project overview", Instruction: "Approach 2", TargetFiles: []string{"src/keep.go"}},
		},
	}

	if _, err := workerEngine.Execute(ctx, chatSession, uuid.NewV7(), uuid.NewV7(), dispatchSignal, ui, nil); err != nil {
		t.Fatalf("WorkerEngine.Execute failed: %v", err)
	}

	if len(ui.reviewCalls) != 1 {
		t.Fatalf("expected 1 bundled ReviewContext call for 2 parallel agents, got %d", len(ui.reviewCalls))
	}
	if ui.reviewCalls[0].Stage != session.ContextStageWorker {
		t.Errorf("expected TurnContext.Stage to be ContextStageWorker, got %v", ui.reviewCalls[0].Stage)
	}
	if ui.reviewCalls[0].PreviewID == "" {
		t.Errorf("expected PreviewID to be populated on bundled worker TurnContext")
	}
	if _, err := service.GetContextPreview(ctx, chatSession, ui.reviewCalls[0].PreviewID); err != nil {
		t.Errorf("expected saved context preview %s to be readable from disk: %v", ui.reviewCalls[0].PreviewID, err)
	}

	if len(recordingClient.capturedBundles) != 2 {
		t.Fatalf("expected 2 worker LLM executions, got %d", len(recordingClient.capturedBundles))
	}
	for i, b := range recordingClient.capturedBundles {
		if len(b.Workbench) != 1 || b.Workbench[0].Path != "src/keep.go" {
			t.Errorf("worker %d expected only retained file 'src/keep.go', got %+v", i+1, b.Workbench)
		}
	}
}

func TestWorkerEngine_AbortContextReview_SpawnsZeroSandboxesAndNoFlowCard(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
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
	writeWorkbenchFile(t, service, "golang", "src/main.go", "package main\n")

	chatSession := &thinkgraph.ChatAddress{
		ChatID:    "chat-worker-abort",
		ProjectID: "golang",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}
	if err := service.InitializeChat(ctx, chatSession); err != nil {
		t.Fatalf("InitializeChat failed: %v", err)
	}

	if err := svcManager.UpdateSpaceSettings("golang", func(s *workspace.SpaceSettings) {
		s.WorkerReviewPolicy.Active = true
		s.WorkerReviewPolicy.MinTokens = 1
	}); err != nil {
		t.Fatalf("UpdateSpaceSettings failed: %v", err)
	}

	registry := config.NewRegistry()
	mockSpace := &weMockThinkSpace{}
	registry.RegisterThinkState("golang", mockSpace)
	registry.RegisterConfig("golang", mockSpace.Config())
	if err := svcManager.UpdateSpaceState("golang", func(s *workspace.SpaceState) { s.ThinkState = "golang" }); err != nil {
		t.Fatalf("UpdateSpaceState failed: %v", err)
	}
	scaffoldChat(t, workspaceRoot, "golang", "chat-worker-abort")

	turnExecutor := session.NewTurnExecutor(logger, &weMockModelClient{}, service)
	fanOutFlow := flows.NewFanOutFlow(logger, flows.FanOutConfig{MaxSteps: 10})

	sessionManager := session.NewManager(logger, registry, svcManager, &weMockModelClient{}, fanOutFlow, nil)
	chatSupervisor, err := sessionManager.GetSession(ctx, "golang", "chat-worker-abort")
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	workerEngine := session.NewWorkerEngine(logger, chatSupervisor, registry, service, turnExecutor, service.Dispatcher(), fanOutFlow)
	ui := &workerReviewCapturingUI{
		weMockUI: weMockUI{reviewToReturn: false},
	}

	dispatchSignal := &signals.SignalDispatchFlow{
		Intention: "Aborted sub-agent batch",
		Tasks: []flows.SubAgentTask{
			{AgentID: "agent-1", Instruction: "Draft", TargetFiles: []string{"src/main.go"}},
		},
	}

	sig, err := workerEngine.Execute(ctx, chatSession, uuid.NewV7(), uuid.NewV7(), dispatchSignal, ui, nil)
	if err != nil {
		t.Fatalf("expected clean terminal signal on user abort, got error: %v", err)
	}
	if !sig.IsTerminal() {
		t.Errorf("expected SignalTerminal on user abort, got %T", sig)
	}
	if engine.spawnCount != 0 {
		t.Errorf("expected zero candidate sandboxes spawned when sub-agent preview was aborted, got %d", engine.spawnCount)
	}
	mainTimeline, err := service.Graph().GetSubGraphTimeline(ctx, chatSession, chatSession.GraphID)
	if err != nil {
		t.Fatalf("GetSubGraphTimeline failed: %v", err)
	}
	if len(mainTimeline) != 0 {
		t.Errorf("expected zero Flow Cards on main timeline when sub-agent preview was aborted, got %d nodes", len(mainTimeline))
	}
}
