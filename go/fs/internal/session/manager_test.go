package session_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/dgraph-io/badger/v4"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/db"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

func TestManager_GetSessionCaching(t *testing.T) {
	ctx := context.Background()
	chatID := "chat-manager-test"
	spaceID := "space-1"

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	workspaceRoot := t.TempDir()

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	graphDB, _ := db.NewBadgerGraph(opts)
	t.Cleanup(func() { graphDB.Close() })

	svcManager := workspace.NewServiceManager(logger, workspaceRoot, func(string) workspace.ChatVersioner { return nil }, graphDB, nil)
	_ = os.MkdirAll(filepath.Join(workspaceRoot, spaceID), 0755)

	// Scaffold required directories
	metaPath := filepath.Join(workspaceRoot, spaceID, "chats", chatID)
	os.MkdirAll(metaPath, 0755)
	os.WriteFile(filepath.Join(metaPath, "meta.json"), []byte(`{"id":"`+chatID+`", "name":"`+chatID+`", "active_branch": "main", "branches": {"main": {"parent_branch": "", "fork_point": ""}}}`), 0644)

	registry := config.NewRegistry()
	manager := session.NewManager(logger, registry, svcManager, nil, nil, nil)

	session1, err := manager.GetSession(ctx, spaceID, chatID)
	if err != nil {
		t.Fatalf("failed to get first session: %v", err)
	}

	session2, err := manager.GetSession(ctx, spaceID, chatID)
	if err != nil {
		t.Fatalf("failed to get second session: %v", err)
	}

	if session1 != session2 {
		t.Errorf("expected Manager to return the exact same cached session pointer for identical space/chat IDs")
	}

	diffChat := "chat-different"
	diffMetaPath := filepath.Join(workspaceRoot, spaceID, "chats", diffChat)
	os.MkdirAll(diffMetaPath, 0755)
	os.WriteFile(filepath.Join(diffMetaPath, "meta.json"), []byte(`{"id":"`+diffChat+`", "name":"`+diffChat+`", "active_branch": "main", "branches": {"main": {"parent_branch": "", "fork_point": ""}}}`), 0644)

	session3, err := manager.GetSession(ctx, spaceID, diffChat)
	if err != nil {
		t.Fatalf("failed to get third session: %v", err)
	}

	if session1 == session3 {
		t.Errorf("expected Manager to return a different session pointer for a different chat ID")
	}
}
