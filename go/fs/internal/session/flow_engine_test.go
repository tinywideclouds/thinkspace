package session_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

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
func (u *feMockChatEngine) ConfirmTransition(targetDomain string, reasoning string) bool {
	return true
}

type feMockSandbox struct {
	candidateID string
	delivered   bool
}

func (m *feMockSandbox) WriteFile(ctx context.Context, path string, data []byte) error { return nil }
func (m *feMockSandbox) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if path == "trace.jsonl" {
		return []byte(`{"prompt": "task", "generated": "output"}`), nil
	}
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

type feMockThinkSpace struct {
	errsToReturn []error
	verifyCalls  int
}

func (m *feMockThinkSpace) Initialize(ctx context.Context, directory string) error { return nil }
func (m *feMockThinkSpace) Config() spaces.ThinkStateConfig                        { return spaces.ThinkStateConfig{} }
func (m *feMockThinkSpace) Patcher() workspace.Patcher                             { return nil }
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

// GetAgentTokenChannel implements [session.UserInterface].
func (ui *feMockUI) GetAgentTokenChannel() chan<- workspace.AgentToken { return nil }

func (u *feMockUI) OnTextChunk(text string) {}

func (u *feMockUI) ReviewContext(ctxData session.TurnContext) session.ContextApprovalDecision {
	return session.ContextApprovalDecision{
		Approved: u.reviewToReturn,
	}
}

func (u *feMockUI) ChooseNextStep() flows.FlowStrategy {
	return u.strategyToReturn
}

func (u *feMockUI) ReviewCandidate(branch string) bool {
	u.reviewedBranch = branch
	return u.reviewToReturn
}

func (u *feMockUI) ConfirmTransition(targetDomain string, reasoning string) bool {
	u.capturedReasoning = reasoning
	return u.transitionToReturn
}

func TestFlowEngine_FanOutIntegration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	engine := &feMockChatEngine{}
	workspaceRoot := t.TempDir()

	ledger := thinkgraph.NewFileLedger(1000, workspaceRoot)
	dispatcher := thinkgraph.NewDispatcher(nil, ledger)

	service := workspace.NewService(logger, engine, workspaceRoot, dispatcher, &feMockEmitter{})
	chatSession, _ := service.InitializeChat(context.Background(), "golang", "chat-1")

	emitter := &feMockEmitter{}
	var bundlesReceived []cognition.ThinkBundle
	executor := func(ctx context.Context, bundle cognition.ThinkBundle, sandbox workspace.CandidateSandbox, agentID int, tokenChan chan<- workspace.AgentToken) error {
		bundlesReceived = append(bundlesReceived, bundle)
		return nil
	}

	flowEngine := session.NewFlowEngine(logger, service, executor, dispatcher, emitter)

	flowCfg := flows.FanOutConfig{MaxSteps: 10, RetryPrompt: "Trace: {{.ErrorTrace}}"}
	fanOutFlow := flows.NewFanOutFlow(logger, flowCfg)
	space := &feMockThinkSpace{}
	ui := &feMockUI{strategyToReturn: flows.StrategyManual, reviewToReturn: true}

	args := map[string]any{
		"agent_tasks": []any{
			map[string]any{
				"context_digest": "Basic server built",
				"instruction":    "Add auth",
				"target_files":   []any{"src/main.go"},
			},
		},
	}
	flowContext := flows.FlowContext{FlowID: uuid.NewV7().String(), SpaceID: "golang", BaseAgentRules: "Worker Persona"}

	err := flowEngine.Run(context.Background(), chatSession, space, fanOutFlow, args, flowContext, ui)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(bundlesReceived) != 1 {
		t.Fatalf("expected 1 execution attempt, got %d", len(bundlesReceived))
	}

	bundle := bundlesReceived[0]
	if bundle.Rules.Persona != "Worker Persona" {
		t.Errorf("expected Worker Persona in rules")
	}
	if len(bundle.Workbench) != 1 || bundle.Workbench[0].Path != "src/main.go" {
		t.Errorf("expected Workbench to materialize target file")
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
	chatSession, _ := service.InitializeChat(context.Background(), "golang", "chat-1")

	flowEngine := session.NewFlowEngine(logger, service, nil, dispatcher, &feMockEmitter{})
	fanOutFlow := flows.NewFanOutFlow(logger, flows.FanOutConfig{MaxSteps: 10})

	args := map[string]any{
		"agent_tasks": []any{
			map[string]any{
				"instruction":  "Edit hallucinated file",
				"target_files": []any{"does/not/exist.go"},
			},
		},
	}

	flowContext := flows.FlowContext{FlowID: uuid.NewV7().String()}
	err := flowEngine.Run(context.Background(), chatSession, &feMockThinkSpace{}, fanOutFlow, args, flowContext, nil)

	if err == nil {
		t.Fatalf("expected flow to fail-fast on missing file, but it succeeded")
	}
	if !strings.Contains(err.Error(), "fatal system errors") {
		t.Errorf("expected fatal system error string, got: %v", err)
	}
}
