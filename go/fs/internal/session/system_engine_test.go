package session_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dgraph-io/badger/v4"
	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/db"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// --- Mocks ---

type seMockChatEngine struct{}

func (e *seMockChatEngine) InitChat(ctx context.Context, chatID string) error { return nil }
func (e *seMockChatEngine) Snapshot(ctx context.Context, chatID, message string) (string, error) {
	return "mock-sha", nil
}
func (e *seMockChatEngine) SpawnCandidateSandbox(ctx context.Context, chatID, candidateID string) (workspace.CandidateSandbox, error) {
	return nil, nil
}
func (e *seMockChatEngine) PreviewCandidate(ctx context.Context, chatID, candidateID string) error {
	return nil
}
func (e *seMockChatEngine) Accept(ctx context.Context, chatID, candidateID, reason string) error {
	return nil
}
func (e *seMockChatEngine) Reject(ctx context.Context, chatID, candidateID, reason string) error {
	return nil
}
func (e *seMockChatEngine) ReadCandidateDiff(ctx context.Context, chatID, candidateID string) (string, error) {
	return "", nil
}

type seMockThinkSpace struct{}

func (s *seMockThinkSpace) Config() spaces.ThinkStateConfig { return spaces.ThinkStateConfig{} }
func (s *seMockThinkSpace) Verifier() spaces.Verifier       { return nil }
func (s *seMockThinkSpace) Mapbook() spaces.Mapbook         { return s }
func (s *seMockThinkSpace) GenerateLayers(ctx context.Context, root string) (map[spaces.MapbookLayer]string, error) {
	return map[spaces.MapbookLayer]string{}, nil
}
func (s *seMockThinkSpace) Patcher() workspace.Patcher                             { return nil }
func (s *seMockThinkSpace) Initialize(ctx context.Context, directory string) error { return nil }
func (s *seMockThinkSpace) EnrichBundle(ctx context.Context, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus, workspaceRoot string) error {
	return nil
}

type seMockUI struct {
	textChunks []string
}

func (ui *seMockUI) OnTextChunk(chunk workspace.NodeChunk) {
	ui.textChunks = append(ui.textChunks, chunk.Text)
}
func (ui *seMockUI) StreamGraphEvent(address *thinkgraph.ChatAddress, node thinkgraph.Node) {}
func (ui *seMockUI) ReviewContext(turnContext session.TurnContext) session.ContextApprovalDecision {
	return session.ContextApprovalDecision{Approved: true}
}
func (u *seMockUI) ChooseNextStep() (flows.FlowStrategy, bool) { return flows.StrategyRefine, true }
func (u *seMockUI) ReviewCandidate(branch string) (bool, string) {
	return false, ""
}
func (ui *seMockUI) ConfirmTransition(targetThinkState string, reasoning string) bool { return true }
func (ui *seMockUI) GetChunkChannel() chan<- workspace.NodeChunk                      { return nil }

type seMockEmitter struct {
	events []telemetry.Event
}

func (e *seMockEmitter) Emit(channel string, event telemetry.Event) {
	e.events = append(e.events, event)
}

// --- Tests ---

func setupSystemEngineTest(t *testing.T) (*session.SystemEngine, *session.ChatSession, *workspace.Service, *config.Registry, string, string) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()
	spaceID := "sys-space"
	chatID := "sys-chat"

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("failed to init badger: %v", err)
	}
	t.Cleanup(func() { graphDB.Close() })

	registry := config.NewRegistry()
	registry.RegisterThinkState("concept", &seMockThinkSpace{})
	registry.RegisterThinkState("golang", &seMockThinkSpace{})

	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatVersioner { return &seMockChatEngine{} }, graphDB, nil)

	_ = os.MkdirAll(filepath.Join(workspaceRoot, spaceID, "chats", chatID), 0755)
	_ = os.WriteFile(filepath.Join(workspaceRoot, spaceID, "chats", chatID, "meta.json"), []byte(`{"id":"`+chatID+`", "name":"Sys Chat", "active_branch": "main"}`), 0644)
	_ = svcManager.UpdateSpaceState(spaceID, func(s *workspace.SpaceState) { s.ThinkState = "concept" })

	chatSession, err := session.NewChatSession(ctx, logger, registry, svcManager, nil, nil, nil, spaceID, chatID)
	if err != nil {
		t.Fatalf("failed to open chat session: %v", err)
	}

	wsService := svcManager.GetService(spaceID)
	systemEngine := session.NewSystemEngine(logger, chatSession, registry, wsService)

	return systemEngine, chatSession, wsService, registry, spaceID, chatID
}

func TestSystemEngine_Execute_SuccessfulTransition(t *testing.T) {
	ctx := context.Background()
	systemEngine, chatSession, wsService, _, spaceID, chatID := setupSystemEngineTest(t)

	// Pre-seed a prompt on the main timeline to act as the driving catalyst
	address := &thinkgraph.ChatAddress{
		ChatID:    chatID,
		ProjectID: spaceID,
		SubGraph: thinkgraph.SubGraph{
			Name:    "main",
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
		},
	}

	catalystEvent := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Let's write some code")
	if err := wsService.Dispatcher().AppendEvent(ctx, address, catalystEvent); err != nil {
		t.Fatalf("Failed to seed catalyst event: %v", err)
	}

	ui := &seMockUI{}
	emitter := &seMockEmitter{}
	turnID := uuid.NewV7()
	catalystID := catalystEvent.ID

	signal := &signals.SignalTransition{
		TargetThinkState: "golang",
	}

	resultSig, err := systemEngine.Execute(ctx, address, turnID, catalystID, signal, ui, emitter)
	if err != nil {
		t.Fatalf("SystemEngine.Execute failed: %v", err)
	}

	// 1. Verify the resulting signal is a clean ExecuteChat intent without string sentinels
	if _, ok := resultSig.(*signals.SignalExecuteChat); !ok {
		t.Errorf("Expected SignalExecuteChat, got %T", resultSig)
	}

	// 2. Verify Session Metadata Updated
	chatSession.Mu().RLock()
	activeThinkState := chatSession.Meta.ActiveThinkState
	chatSession.Mu().RUnlock()

	if activeThinkState != "golang" {
		t.Errorf("Expected session ActiveThinkState to be 'golang', got '%s'", activeThinkState)
	}

	// 3. Verify Administrative System Subgraph Isolation
	sysAddr := chatSession.GetSystemAddress()
	if sysAddr.Name != "system" {
		t.Errorf("Expected system subgraph to be named 'system', got '%s'", sysAddr.Name)
	}

	sysTimeline, err := wsService.Graph().GetSubGraphTimeline(ctx, sysAddr, sysAddr.GraphID)
	if err != nil {
		t.Fatalf("Failed to fetch system timeline: %v", err)
	}

	foundSystemEvent := false
	for _, n := range sysTimeline {
		if n.Type == thinkgraph.EventSystem && strings.Contains(n.Content, "Workspace transitioned to golang") {
			foundSystemEvent = true
			break
		}
	}
	if !foundSystemEvent {
		t.Errorf("Expected transition EventSystem node to be dual-written to 'system' subgraph")
	}

	// 4. Verify Catalyst Prompt Mutation (Dual-Write Inline Badge)
	mainTimeline, err := wsService.Graph().GetSubGraphTimeline(ctx, address, address.GraphID)
	if err != nil {
		t.Fatalf("Failed to fetch main timeline: %v", err)
	}

	for _, n := range mainTimeline {
		if n.Type == thinkgraph.EventSystem {
			t.Errorf("Forbidden EventSystem node found polluting main conversation timeline: %s", n.Content)
		}
		if n.ID == catalystID {
			if n.Metadata["transitioned_to"] != "golang" {
				t.Errorf("Expected catalyst prompt to receive 'transitioned_to: golang' metadata badge, got: %v", n.Metadata)
			}
		}
	}
}

func TestSystemEngine_Execute_InvalidTarget(t *testing.T) {
	ctx := context.Background()
	systemEngine, _, _, _, _, _ := setupSystemEngineTest(t)

	address := &thinkgraph.ChatAddress{
		ChatID:    "sys-chat",
		ProjectID: "sys-space",
		SubGraph: thinkgraph.SubGraph{
			Name: "main",
		},
	}

	ui := &seMockUI{}
	signal := &signals.SignalTransition{
		TargetThinkState: "non-existent-domain",
	}

	resultSig, err := systemEngine.Execute(ctx, address, uuid.NewV7(), uuid.NewV7(), signal, ui, nil)

	if err == nil {
		t.Fatalf("Expected error for non-existent think state transition")
	}
	if resultSig != nil {
		t.Errorf("Expected nil signal on error, got %T", resultSig)
	}

	// Check if the UI received the system error chunk
	if len(ui.textChunks) == 0 || !strings.Contains(ui.textChunks[0], "Think state 'non-existent-domain' does not exist") {
		t.Errorf("Expected UI to receive inline error chunk, got %v", ui.textChunks)
	}
}

func TestSystemEngine_Execute_InvalidSignalType(t *testing.T) {
	ctx := context.Background()
	systemEngine, _, _, _, _, _ := setupSystemEngineTest(t)

	address := &thinkgraph.ChatAddress{}

	// Pass an incorrect signal type
	wrongSignal := &signals.SignalSynthesize{}
	_, err := systemEngine.Execute(ctx, address, uuid.NewV7(), uuid.NewV7(), wrongSignal, &seMockUI{}, nil)

	if err == nil || !strings.Contains(err.Error(), "expected SignalTransition") {
		t.Errorf("Expected specific type error, got %v", err)
	}
}
