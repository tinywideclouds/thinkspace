package session_test

import (
	"context"
	"errors"
	"iter"
	"log/slog"
	"os"
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
	sandbox      *weMockSandbox
	acceptCalled bool
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
	m.acceptCalled = true
	return nil
}
func (m *weMockChatEngine) Reject(ctx context.Context, chatID string, candidateID string, reason string) error {
	return nil
}
func (m *weMockChatEngine) SpawnCandidateSandbox(ctx context.Context, chatID string, candidateID string) (workspace.CandidateSandbox, error) {
	m.sandbox = &weMockSandbox{candidateID: candidateID}
	return m.sandbox, nil
}

type weMockSandbox struct {
	candidateID string
	delivered   bool
}

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

func (p *weMockPatcher) Apply(ctx context.Context, sandbox workspace.CandidateSandbox, llmOutput string) error {
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
func (m *weMockThinkSpace) Verify(ctx context.Context, sandbox workspace.CandidateSandbox) error {
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
	reviewToReturn     bool
	reviewedBranch     string
	transitionToReturn bool
	capturedReasoning  string
}

func (u *weMockUI) OnTextChunk(chunk workspace.NodeChunk)                                  {}
func (u *weMockUI) StreamGraphEvent(address *thinkgraph.ChatAddress, node thinkgraph.Node) {}
func (u *weMockUI) ReviewContext(ctxData session.TurnContext) session.ContextApprovalDecision {
	return session.ContextApprovalDecision{Approved: u.reviewToReturn}
}
func (u *weMockUI) ChooseNextStep() flows.FlowStrategy { return u.strategyToReturn }
func (u *weMockUI) ReviewCandidate(branch string) bool {
	u.reviewedBranch = branch
	return u.reviewToReturn
}
func (u *weMockUI) ConfirmTransition(targetThinkState string, reasoning string) bool {
	u.capturedReasoning = reasoning
	return u.transitionToReturn
}
func (u *weMockUI) GetChunkChannel() chan<- workspace.NodeChunk { return nil }

func TestWorkerEngine_FanOutIntegration_TopologyAndEdges(t *testing.T) {
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
		ChatID:    "chat-1",
		ProjectID: "golang",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}
	_ = service.InitializeChat(context.Background(), chatSession)

	registry := config.NewRegistry()
	mockSpace := &weMockThinkSpace{}
	registry.RegisterThinkState("golang", mockSpace)
	registry.RegisterConfig("golang", mockSpace.Config())
	_ = svcManager.UpdateSpaceState("golang", func(s *workspace.SpaceState) { s.ThinkState = "golang" })

	scaffoldChat(workspaceRoot, "golang", "chat-1")

	modelClient := &weMockModelClient{
		responses: []llm.StreamChunk{{Text: "Sub-agent output."}},
	}
	turnExecutor := session.NewTurnExecutor(logger, modelClient, service, emitter)

	flowCfg := flows.FanOutConfig{MaxSteps: 10, RetryPrompt: "Trace: {{.ErrorTrace}}"}
	fanOutFlow := flows.NewFanOutFlow(logger, flowCfg)

	sessionManager := session.NewManager(logger, registry, svcManager, modelClient, fanOutFlow, nil)
	chatSupervisor, err := sessionManager.GetSession(context.Background(), "golang", "chat-1")
	if err != nil {
		t.Fatalf("failed to get supervisor session: %v", err)
	}

	workerEngine := session.NewWorkerEngine(logger, chatSupervisor, registry, service, turnExecutor, dispatcher, fanOutFlow)
	ui := &weMockUI{strategyToReturn: flows.StrategyRefine, reviewToReturn: true}

	tasks := []flows.SubAgentTask{
		{
			AgentID:       "agent-1",
			ContextDigest: "Basic server built",
			Instruction:   "Add auth",
			TargetFiles:   []string{"src/main.go"},
		},
	}

	dispatchSignal := &signals.SignalDispatchFlow{
		Tasks: tasks,
	}

	turnID := uuid.NewV7()
	catalystID := uuid.NewV7()

	signal, err := workerEngine.Execute(context.Background(), chatSession, turnID, catalystID, dispatchSignal, ui, emitter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	synthSig, ok := signal.(*signals.SignalSynthesize)
	if !ok {
		t.Fatalf("Expected SignalSynthesize based on StrategyRefine yield, got %T", signal)
	}

	if synthSig.FlowAddress == nil {
		t.Errorf("Expected synthesise signal to carry active FlowAddress for cross-boundary routing")
	}

	mainTimeline, _ := service.Graph().GetSubGraphTimeline(context.Background(), chatSession, chatSession.GraphID)
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

	if optionsNode == nil {
		t.Fatalf("Expected Flow Options Node on main timeline")
	}

	if synthSig.ParentNodeID != optionsNode.ID {
		t.Errorf("Expected yielded signal ParentNodeID to match the Flow Options node ID")
	}

	subGraphAddr := synthSig.FlowAddress
	_ = service.SyncGraphContext(context.Background(), subGraphAddr)
	flowTimeline, _ := service.Graph().GetSubGraphTimeline(context.Background(), subGraphAddr, subGraphAddr.GraphID)

	var taskNode *thinkgraph.Node
	var joinNode *thinkgraph.Node
	for _, n := range flowTimeline {
		if n.Metadata["agent_id"] == "agent-1" && n.Type == thinkgraph.EventPrompt {
			nodeCopy := n
			taskNode = &nodeCopy
		} else if n.Type == flows.EventFlowLifecycle {
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

func TestWorkerEngine_FailFast_HallucinatedFile(t *testing.T) {
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
	_ = service.InitializeChat(context.Background(), chatSession)

	registry := config.NewRegistry()
	mockSpace := &weMockThinkSpace{}
	registry.RegisterThinkState("golang", mockSpace)
	registry.RegisterConfig("golang", mockSpace.Config())
	_ = svcManager.UpdateSpaceState("golang", func(s *workspace.SpaceState) { s.ThinkState = "golang" })

	scaffoldChat(workspaceRoot, "golang", "chat-1")

	turnExecutor := session.NewTurnExecutor(logger, &weMockModelClient{}, service, emitter)
	fanOutFlow := flows.NewFanOutFlow(logger, flows.FanOutConfig{MaxSteps: 10})

	sessionManager := session.NewManager(logger, registry, svcManager, &weMockModelClient{}, fanOutFlow, nil)
	chatSupervisor, err := sessionManager.GetSession(context.Background(), "golang", "chat-1")
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

	_, err = workerEngine.Execute(context.Background(), chatSession, uuid.NewV7(), uuid.NewV7(), dispatchSignal, &weMockUI{}, emitter)

	if err == nil {
		t.Fatalf("expected flow to fail-fast on missing file, but it succeeded")
	}
	if !strings.Contains(err.Error(), "fatal system errors") {
		t.Errorf("expected fatal system error string, got: %v", err)
	}
}
