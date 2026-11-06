package session_test

import (
	"context"
	"errors"
	"iter"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
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

type feMockModelClient struct {
	responses []llm.StreamChunk
}

func (client *feMockModelClient) GenerateContentStream(
	ctx context.Context,
	model string,
	bundle cognition.ThinkBundle,
	registry *tools.Registry,
	config spaces.ThinkStateConfig,
) iter.Seq2[llm.StreamChunk, error] {
	return func(yield func(llm.StreamChunk, error) bool) {
		for _, resp := range client.responses {
			yield(resp, nil)
		}
	}
}

type feMockChatEngine struct {
	sandbox      *feMockSandbox
	acceptCalled bool
}

func (m *feMockChatEngine) InitChat(ctx context.Context, chatID string) error { return nil }
func (m *feMockChatEngine) Snapshot(ctx context.Context, chatID string, msg string) (string, error) {
	return "", nil
}
func (m *feMockChatEngine) PreviewCandidate(ctx context.Context, chatID string, candidateID string) error {
	return nil
}
func (m *feMockChatEngine) ReadCandidateDiff(ctx context.Context, chatID string, candidateID string) (string, error) {
	return "+ mock diff", nil
}
func (m *feMockChatEngine) Accept(ctx context.Context, chatID string, candidateID string, reason string) error {
	m.acceptCalled = true
	return nil
}
func (m *feMockChatEngine) Reject(ctx context.Context, chatID string, candidateID string, reason string) error {
	return nil
}
func (m *feMockChatEngine) SpawnCandidateSandbox(ctx context.Context, chatID string, candidateID string) (workspace.CandidateSandbox, error) {
	m.sandbox = &feMockSandbox{candidateID: candidateID}
	return m.sandbox, nil
}

type feMockSandbox struct {
	candidateID string
	delivered   bool
}

func (m *feMockSandbox) WriteFile(ctx context.Context, path string, data []byte) error { return nil }
func (m *feMockSandbox) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if path == "does/not/exist.go" {
		return nil, errors.New("file not found")
	}
	return []byte("mock file content"), nil
}
func (m *feMockSandbox) ExecuteCommand(ctx context.Context, cmd string, args ...string) (string, error) {
	return "", nil
}
func (m *feMockSandbox) ApplyDraft(ctx context.Context, message string) error { return nil }
func (m *feMockSandbox) DeliverForReview(ctx context.Context) error           { m.delivered = true; return nil }
func (m *feMockSandbox) TearDown(ctx context.Context) error                   { return nil }

type feMockPatcher struct{}

func (p *feMockPatcher) Apply(ctx context.Context, sandbox workspace.CandidateSandbox, llmOutput string) error {
	return nil
}
func (p *feMockPatcher) SystemInstructions() string { return "" }

type feMockThinkSpace struct {
	errsToReturn []error
	verifyCalls  int
}

func (m *feMockThinkSpace) Initialize(ctx context.Context, directory string) error { return nil }
func (m *feMockThinkSpace) Config() spaces.ThinkStateConfig                        { return spaces.ThinkStateConfig{} }
func (m *feMockThinkSpace) Patcher() workspace.Patcher                             { return &feMockPatcher{} }
func (m *feMockThinkSpace) Verifier() spaces.Verifier                              { return m }
func (m *feMockThinkSpace) Verify(ctx context.Context, sandbox workspace.CandidateSandbox) error {
	if m.verifyCalls < len(m.errsToReturn) {
		err := m.errsToReturn[m.verifyCalls]
		m.verifyCalls++
		return err
	}
	m.verifyCalls++
	return nil
}
func (m *feMockThinkSpace) Mapbook() spaces.Mapbook { return m }
func (m *feMockThinkSpace) GenerateLayers(ctx context.Context, root string) (map[spaces.MapbookLayer]string, error) {
	return map[spaces.MapbookLayer]string{spaces.LayerStructural: "Mock Tree"}, nil
}
func (m *feMockThinkSpace) EnrichBundle(ctx context.Context, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus, workspaceRoot string) error {
	return nil
}

type feMockEmitter struct {
	events []telemetry.Event
}

func (m *feMockEmitter) Emit(channel string, event telemetry.Event) {
	m.events = append(m.events, event)
}

type feMockUI struct {
	strategyToReturn   flows.FlowStrategy
	reviewToReturn     bool
	reviewedBranch     string
	transitionToReturn bool
	capturedReasoning  string
}

func (ui *feMockUI) GetAgentTokenChannel() chan<- workspace.AgentToken { return nil }
func (u *feMockUI) OnTextChunk(text string)                            {}
func (u *feMockUI) ReviewContext(ctxData session.TurnContext) session.ContextApprovalDecision {
	return session.ContextApprovalDecision{Approved: u.reviewToReturn}
}
func (u *feMockUI) ChooseNextStep() flows.FlowStrategy { return u.strategyToReturn }
func (u *feMockUI) ReviewCandidate(branch string) bool {
	u.reviewedBranch = branch
	return u.reviewToReturn
}
func (u *feMockUI) ConfirmTransition(req signals.SignalTransition) bool {
	u.capturedReasoning = req.Reasoning
	return u.transitionToReturn
}

func TestFlowEngine_FanOutIntegration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	engine := &feMockChatEngine{}
	workspaceRoot := t.TempDir()

	ledger := thinkgraph.NewFileLedger(1000, workspaceRoot)
	dispatcher := thinkgraph.NewDispatcher(nil, ledger)

	service := workspace.NewService(logger, engine, workspaceRoot, dispatcher, &feMockEmitter{})

	chatSession := &thinkgraph.ChatAddress{
		ChatID:    "chat-1",
		ProjectID: "golang",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}
	_ = service.InitializeChat(context.Background(), chatSession)

	emitter := &feMockEmitter{}
	modelClient := &feMockModelClient{
		responses: []llm.StreamChunk{{Text: "Sub-agent implemented the feature."}},
	}
	turnExecutor := session.NewTurnExecutor(logger, modelClient, service, emitter)

	flowEngine := session.NewFlowEngine(logger, service, turnExecutor, dispatcher)

	flowCfg := flows.FanOutConfig{MaxSteps: 10, RetryPrompt: "Trace: {{.ErrorTrace}}"}
	fanOutFlow := flows.NewFanOutFlow(logger, flowCfg)
	space := &feMockThinkSpace{}
	ui := &feMockUI{strategyToReturn: flows.StrategyManual, reviewToReturn: true}

	tasks := []flows.SubAgentTask{
		{
			ContextDigest: "Basic server built",
			Instruction:   "Add auth",
			TargetFiles:   []string{"src/main.go"},
		},
	}

	signal, err := flowEngine.Run(context.Background(), chatSession, space, fanOutFlow, tasks, "Executing auth flow", ui, emitter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := signal.(*signals.SignalTerminal); !ok {
		t.Errorf("Expected SignalTerminal, got %T", signal)
	}

	if !engine.sandbox.delivered {
		t.Errorf("expected sandbox to be delivered")
	}
	if !engine.acceptCalled {
		t.Errorf("expected candidate to be accepted by UI mock")
	}
}

func TestFlowEngine_FailFast_HallucinatedFile(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	engine := &feMockChatEngine{}
	workspaceRoot := t.TempDir()

	ledger := thinkgraph.NewFileLedger(1000, workspaceRoot)
	dispatcher := thinkgraph.NewDispatcher(nil, ledger)

	service := workspace.NewService(logger, engine, workspaceRoot, dispatcher, &feMockEmitter{})
	chatSession := &thinkgraph.ChatAddress{
		ChatID:    "chat-1",
		ProjectID: "golang",
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}
	_ = service.InitializeChat(context.Background(), chatSession)

	turnExecutor := session.NewTurnExecutor(logger, &feMockModelClient{}, service, &feMockEmitter{})
	flowEngine := session.NewFlowEngine(logger, service, turnExecutor, dispatcher)
	fanOutFlow := flows.NewFanOutFlow(logger, flows.FanOutConfig{MaxSteps: 10})

	tasks := []flows.SubAgentTask{
		{
			Instruction: "Edit hallucinated file",
			TargetFiles: []string{"does/not/exist.go"},
		},
	}

	_, err := flowEngine.Run(context.Background(), chatSession, &feMockThinkSpace{}, fanOutFlow, tasks, "test flow", nil, &feMockEmitter{})

	if err == nil {
		t.Fatalf("expected flow to fail-fast on missing file, but it succeeded")
	}
	if !strings.Contains(err.Error(), "fatal system errors") {
		t.Errorf("expected fatal system error string, got: %v", err)
	}
}
