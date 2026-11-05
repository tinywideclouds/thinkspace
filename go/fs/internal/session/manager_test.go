package session_test

import (
	"context"
	"testing"
)

func TestManager_GetSessionCaching(t *testing.T) {
	ctx := context.Background()
	chatID := "chat-manager-test"

	// Rely on the shared setupSessionTest fixture defined in chat_session_test.go
	manager, _, svcManager, _, _, spaceID := setupSessionTest(t, chatID)

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

	// Scaffold the secondary chat directory so it can be loaded
	scaffoldTestChat(svcManager.WorkspaceRoot(), spaceID, "chat-different")

	session3, err := manager.GetSession(ctx, spaceID, "chat-different")
	if err != nil {
		t.Fatalf("failed to get third session: %v", err)
	}

	if session1 == session3 {
		t.Errorf("expected Manager to return a different session pointer for a different chat ID")
	}
}
