package session_test

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"os"
	"testing"

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

type seTestModelClient struct {
	responses []llm.StreamChunk
}

func (client *seTestModelClient) GenerateContentStream(
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

type seTestChatEngine struct {
	sandbox *seTestSandbox
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
	return nil
}
func (m *seTestChatEngine) Reject(ctx context.Context, chatID string, candidateID string, reason string) error {
	return nil
}
func (m *seTestChatEngine) SpawnCandidateSandbox(ctx context.Context, chatID string, candidateID string) (workspace.CandidateSandbox, error) {
	m.sandbox = &seTestSandbox{candidateID: candidateID}
	return m.sandbox, nil
}

type seTestSandbox struct {
	candidateID string
}

func (m *seTestSandbox) WriteFile(ctx context.Context, path string, data []byte) error { return nil }
func (m *seTestSandbox) ReadFile(ctx context.Context, path string) ([]byte, error) {
	return []byte("mock file content"), nil
}
func (m *seTestSandbox) ExecuteCommand(ctx context.Context, cmd string, args ...string) (string, error) {
	return "", nil
}
func (m *seTestSandbox) ApplyDraft(ctx context.Context, message string) error { return nil }
func (m *seTestSandbox) DeliverForReview(ctx context.Context) error           { return nil }
func (m *seTestSandbox) TearDown(ctx context.Context) error                   { return nil }

type seTestThinkSpace struct{}

func (m *seTestThinkSpace) Initialize(ctx context.Context, directory string) error { return nil }
func (m *seTestThinkSpace) Config() spaces.ThinkStateConfig {
	return spaces.ThinkStateConfig{}
}
func (m *seTestThinkSpace) Patcher() workspace.Patcher { return nil }
func (m *seTestThinkSpace) Verifier() spaces.Verifier  { return m }
func (m *seTestThinkSpace) Verify(ctx context.Context, sandbox workspace.CandidateSandbox) error {
	return nil
}
func (m *seTestThinkSpace) Mapbook() spaces.Mapbook { return m }
func (m *seTestThinkSpace) GenerateLayers(ctx context.Context, root string) (map[spaces.MapbookLayer]string, error) {
	return map[spaces.MapbookLayer]string{}, nil
}
func (m *seTestThinkSpace) EnrichBundle(ctx context.Context, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus, workspaceRoot string) error {
	return nil
}

type seTestUI struct {
	reviewToReturn bool
}

func (u *seTestUI) OnTextChunk(chunk workspace.NodeChunk)                                  {}
func (u *seTestUI) StreamGraphEvent(address *thinkgraph.ChatAddress, node thinkgraph.Node) {}
func (u *seTestUI) ReviewContext(ctxData session.TurnContext) session.ContextApprovalDecision {
	return session.ContextApprovalDecision{}
}
func (u *seTestUI) ChooseNextStep() flows.FlowStrategy { return flows.StrategyRefine }
func (u *seTestUI) ReviewCandidate(branch string) bool {
	return u.reviewToReturn
}
func (u *seTestUI) ConfirmTransition(req signals.SignalTransition) bool { return true }
func (u *seTestUI) GetChunkChannel() chan<- workspace.NodeChunk         { return nil }

type seTestEmitter struct{}

func (m *seTestEmitter) Emit(channel string, event telemetry.Event) {}

func TestSynthesizeEngine_TopologyAndEdges(t *testing.T) {
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
	_ = service.InitializeChat(context.Background(), chatSession)

	registry := config.NewRegistry()
	mockSpace := &seTestThinkSpace{}
	registry.RegisterThinkState("golang", mockSpace)
	registry.RegisterConfig("golang", mockSpace.Config())
	_ = svcManager.UpdateSpaceState("golang", func(s *workspace.SpaceState) { s.ThinkState = "golang" })

	scaffoldChat(workspaceRoot, "golang", "chat-1")

	modelClient := &seTestModelClient{
		responses: []llm.StreamChunk{{Text: "Synthesized golden output."}},
	}
	turnExecutor := session.NewTurnExecutor(logger, modelClient, service, emitter)

	sessionManager := session.NewManager(logger, registry, svcManager, modelClient, nil, nil)
	chatSupervisor, err := sessionManager.GetSession(context.Background(), "golang", "chat-1")
	if err != nil {
		t.Fatalf("failed to get supervisor session: %v", err)
	}

	synthEngine := session.NewSynthesizeEngine(logger, registry, service, turnExecutor, chatSupervisor)
	ui := &seTestUI{reviewToReturn: true}

	flowID := uuid.NewV7()
	optionsNodeID := uuid.NewV7()
	flowAddress := &thinkgraph.ChatAddress{
		ChatID:    "chat-1",
		ProjectID: "golang",
		SubGraph: thinkgraph.SubGraph{
			Name:    fmt.Sprintf("flow-%s", flowID.String()),
			GraphID: thinkgraph.GraphID{Prefix: thinkgraph.PrefixFlow, UUID: flowID},
		},
	}

	synthSignal := &signals.SignalSynthesize{
		OriginalInstruction: "Do work",
		FlowAddress:         flowAddress,
		ParentNodeID:        optionsNodeID,
	}

	turnID := uuid.NewV7()
	signal, err := synthEngine.Execute(context.Background(), chatSession, turnID, uuid.NewV7(), synthSignal, ui, emitter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := signal.(*signals.SignalTerminal); !ok {
		t.Errorf("Expected SignalTerminal from synthesis, got %T", signal)
	}

	_ = service.SyncGraphContext(context.Background(), flowAddress)
	timeline, _ := service.Graph().GetSubGraphTimeline(context.Background(), flowAddress, flowAddress.GraphID)

	if len(timeline) != 3 {
		t.Fatalf("Expected strictly 3 synthesis nodes (Prompt, Model, Resolution) in subgraph, got %d", len(timeline))
	}

	synthPrompt := timeline[0]
	if synthPrompt.Type != thinkgraph.EventPrompt {
		t.Errorf("Expected first node to be EventPrompt, got %s", synthPrompt.Type)
	}
	if synthPrompt.Edges["parent_id"] != optionsNodeID.String() {
		t.Errorf("Expected Synthesis Prompt to point cross-boundary to OptionsNodeID (%s), got %v", optionsNodeID.String(), synthPrompt.Edges)
	}

	synthModel := timeline[1]
	if synthModel.Type != thinkgraph.EventModel {
		t.Errorf("Expected second node to be EventModel, got %s", synthModel.Type)
	}
	if synthModel.Edges["parent_id"] != synthPrompt.ID.String() {
		t.Errorf("Expected Synthesis Model to point to Synthesis Prompt")
	}

	synthRes := timeline[2]
	if synthRes.Type != workspace.EventCandidateResolution {
		t.Errorf("Expected third node to be CandidateResolution, got %s", synthRes.Type)
	}
	if synthRes.Edges["parent_id"] != synthModel.ID.String() {
		t.Errorf("Expected Synthesis Resolution to point to Synthesis Model")
	}
}
