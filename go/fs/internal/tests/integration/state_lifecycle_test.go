package integration_test

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/db"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// mockChatEngine just stubs the git layer so we don't need a real git repo
type mockChatEngine struct{}

func (m *mockChatEngine) InitChat(ctx context.Context, chatID string) error { return nil }
func (m *mockChatEngine) Snapshot(ctx context.Context, chatID, msg string) (string, error) {
	return "", nil
}
func (m *mockChatEngine) SpawnCandidateSandbox(ctx context.Context, chatID, candidateID string) (workspace.CandidateSandbox, error) {
	return nil, nil
}
func (m *mockChatEngine) PreviewCandidate(ctx context.Context, chatID, candidateID string) error {
	return nil
}
func (m *mockChatEngine) Accept(ctx context.Context, chatID, candidateID, reason string) error {
	return nil
}
func (m *mockChatEngine) Reject(ctx context.Context, chatID, candidateID, reason string) error {
	return nil
}
func (m *mockChatEngine) ReadCandidateDiff(ctx context.Context, chatID, candidateID string) (string, error) {
	return "", nil
}

// mockDeterministicLLM returns exactly what we tell it to.
type mockDeterministicLLM struct {
	Response string
}

func (m *mockDeterministicLLM) GenerateContentStream(ctx context.Context, model string, bundle cognition.ThinkBundle, tools []llm.ToolDef) iter.Seq2[llm.StreamChunk, error] {
	return func(yield func(llm.StreamChunk, error) bool) {
		yield(llm.StreamChunk{Text: m.Response}, nil)
	}
}

func TestEndToEndStateLifecycle(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()
	spaceID := "integration-space"
	chatID := "lifecycle-chat"

	// 1. Setup the Graph DB
	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, err := db.NewBadgerGraph(opts)
	if err != nil {
		t.Fatalf("Failed to init BadgerDB: %v", err)
	}
	defer graphDB.Close()

	// 2. Setup the Bus & Service
	bus := thinkgraph.NewEventBus()
	bus.Subscribe(thinkgraph.NewLedgerSubscriber(1000, workspaceRoot))
	bus.Subscribe(thinkgraph.NewDigestSubscriber(workspaceRoot))
	bus.Subscribe(thinkgraph.NewGraphSubscriber(graphDB))

	svc := workspace.NewService(logger, &mockChatEngine{}, workspaceRoot, bus, graphDB)
	chatSession, err := svc.InitializeChat(ctx, spaceID, chatID)
	if err != nil {
		t.Fatalf("Failed to initialize chat: %v", err)
	}

	var targetEvents []thinkgraph.Event

	t.Run("Act I: The Conversation", func(t *testing.T) {
		if err := svc.LogUserPrompt(ctx, chatSession, "How do I build a server?"); err != nil {
			t.Fatalf("LogUserPrompt failed: %v", err)
		}
		time.Sleep(1 * time.Millisecond) // Ensure deterministic sorting
		if err := svc.LogModelResponse(ctx, chatSession, "Use net/http.", nil); err != nil {
			t.Fatalf("LogModelResponse failed: %v", err)
		}

		// Fetch the raw events from graph to get their UUIDs
		timeline, err := graphDB.GetBranchTimeline(ctx, chatSession, chatSession.ActiveBranch)
		if err != nil {
			t.Fatalf("GetBranchTimeline failed: %v", err)
		}
		if len(timeline) != 2 {
			t.Fatalf("Expected 2 events in graph, got %d", len(timeline))
		}
		targetEvents = timeline
	})

	t.Run("Act II: The Compression", func(t *testing.T) {
		// Mock an LLM that explicitly summarizes those two specific events
		mockJSON := fmt.Sprintf(`{
			"thought": "Summarizing the server setup",
			"digests": [
				{
					"topic_name": "Server Setup",
					"tags": ["golang", "http"],
					"summary": "User asked how to build a server. Model suggested net/http."
				}
			]
		}`)

		mockLLM := &mockDeterministicLLM{Response: mockJSON}
		digestManager := session.NewDigestManager(logger, mockLLM, session.DigestConfig{})

		// Note: CreateBackgroundDigest embeds the IDs of targetEvents into the DigestMeta
		count, err := digestManager.CreateBackgroundDigest(ctx, svc, chatSession, targetEvents, nil, true, "mock-model", "persona")
		if err != nil || count != 1 {
			t.Fatalf("Failed to create digest: %v", err)
		}

		// Verify Dual Write occurred
		digestsDir := filepath.Join(workspaceRoot, spaceID, "chats", chatID, "ledger", "digests")
		entries, err := os.ReadDir(digestsDir)
		if err != nil {
			t.Fatalf("Failed to read digests directory: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("Expected 1 markdown digest on disk, got %d", len(entries))
		}

		// Verify Graph
		graphDigests, err := graphDB.GetDigests(ctx, chatSession)
		if err != nil {
			t.Fatalf("GetDigests failed: %v", err)
		}
		if len(graphDigests) != 1 {
			t.Fatalf("Expected 1 digest in graph, got %d", len(graphDigests))
		}
	})

	t.Run("Act III: Amnesia & Rehydration", func(t *testing.T) {
		// 1. Wipe the graph
		if err := graphDB.ClearChatGraph(ctx, chatSession); err != nil {
			t.Fatalf("ClearChatGraph failed: %v", err)
		}
		timeline, err := graphDB.GetBranchTimeline(ctx, chatSession, chatSession.ActiveBranch)
		if err != nil {
			t.Fatalf("GetBranchTimeline after clear failed: %v", err)
		}
		if len(timeline) != 0 {
			t.Fatalf("Failed to wipe graph: %d events remain", len(timeline))
		}

		// 2. Rehydrate from Disk
		rehydrator := thinkgraph.NewRehydrator(graphDB, workspaceRoot)
		if err := rehydrator.Rebuild(ctx, chatSession); err != nil {
			t.Fatalf("Rehydration failed: %v", err)
		}

		// 3. Verify Timeline Integrity
		restoredTimeline, err := graphDB.GetBranchTimeline(ctx, chatSession, chatSession.ActiveBranch)
		if err != nil {
			t.Fatalf("GetBranchTimeline after rehydration failed: %v", err)
		}
		if len(restoredTimeline) != 2 {
			t.Fatalf("Expected 2 events restored, got %d", len(restoredTimeline))
		}
		if restoredTimeline[0].ID != targetEvents[0].ID {
			t.Errorf("Timeline ordering corrupted during rehydration")
		}

		// 4. Verify Digest & Overlapping Linkage Integrity
		restoredDigests, err := graphDB.GetDigests(ctx, chatSession)
		if err != nil {
			t.Fatalf("GetDigests after rehydration failed: %v", err)
		}
		if len(restoredDigests) != 1 {
			t.Fatalf("Expected 1 digest restored, got %d", len(restoredDigests))
		}

		restoredDigest := restoredDigests[0]
		if restoredDigest.Summary != "User asked how to build a server. Model suggested net/http." {
			t.Errorf("Digest summary corrupted")
		}

		// CRITICAL: Did the markdown frontmatter correctly restore the linked Event UUIDs?
		if len(restoredDigest.SummarizedEvents) != 2 {
			t.Fatalf("Expected 2 linked events, got %d", len(restoredDigest.SummarizedEvents))
		}
		if restoredDigest.SummarizedEvents[0] != targetEvents[0].ID {
			t.Errorf("Linkage lost: Expected %s, got %s", targetEvents[0].ID, restoredDigest.SummarizedEvents[0])
		}
	})
}
