package gitfs_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/tinywideclouds.com/thinkspace/internal/gitfs"
)

func setupTestEnvironment(t *testing.T) (context.Context, string, *gitfs.NativeEngine, string, string) {
	ctx := context.Background()
	dir := t.TempDir()
	engine := gitfs.NewNativeEngine()

	threadID := "test-thread"
	proposalID := "test-proposal"

	if err := engine.InitThread(ctx, dir, threadID); err != nil {
		t.Fatalf("InitThread failed: %v", err)
	}

	chatsDir := filepath.Join(dir, "chats", threadID)
	if err := os.MkdirAll(chatsDir, 0755); err != nil {
		t.Fatalf("failed to create chats dir: %v", err)
	}

	return ctx, dir, engine, threadID, proposalID
}

func TestStateEngine_Lifecycle_Reject(t *testing.T) {
	ctx, dir, engine, threadID, proposalID := setupTestEnvironment(t)
	chatsDir := filepath.Join(dir, "chats", threadID)

	// Create floating ledger
	ledgerPath := filepath.Join(chatsDir, "conversation.jsonl")
	uncommittedText := "user: create a math function\n"
	if err := os.WriteFile(ledgerPath, []byte(uncommittedText), 0644); err != nil {
		t.Fatalf("failed to write floating ledger: %v", err)
	}

	// Propose candidate
	files := map[string][]byte{
		"math.go": []byte("package math\n"),
	}
	commitHash, err := engine.Propose(ctx, dir, threadID, proposalID, files, "test-tool")
	if err != nil {
		t.Fatalf("Propose failed: %v", err)
	}
	if commitHash == "" {
		t.Fatal("expected a valid commit hash")
	}

	// VERIFY: We must be checked out on the candidate branch
	repo, _ := git.PlainOpen(dir)
	head, _ := repo.Head()
	if expected := "refs/heads/candidate/" + proposalID; head.Name().String() != expected {
		t.Errorf("Expected branch %s, got %s", expected, head.Name().String())
	}

	// Reject candidate
	if err := engine.Reject(ctx, dir, threadID, proposalID); err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	// VERIFY A: Safely returned to thread branch
	head, _ = repo.Head()
	if expected := "refs/heads/chat/" + threadID; head.Name().String() != expected {
		t.Errorf("Expected branch %s, got %s", expected, head.Name().String())
	}

	// VERIFY B: Ledger floated safely back
	content, err := os.ReadFile(ledgerPath)
	if err != nil || string(content) != uncommittedText {
		t.Errorf("Uncommitted ledger was lost! Content: %s", string(content))
	}

	// VERIFY C: Permanent tag survives
	tagRefName := plumbing.ReferenceName("refs/tags/proposal-" + proposalID)
	if _, err := repo.Reference(tagRefName, true); err != nil {
		t.Errorf("Proposal tag was destroyed: %v", err)
	}

	// VERIFY D: Proposed file cleanly REMOVED from disk
	mathPath := filepath.Join(chatsDir, "docs", "math.go")
	if _, err := os.Stat(mathPath); !os.IsNotExist(err) {
		t.Errorf("Proposed file %s was NOT removed after rejection", mathPath)
	}
}

func TestStateEngine_Lifecycle_Accept(t *testing.T) {
	ctx, dir, engine, threadID, proposalID := setupTestEnvironment(t)
	chatsDir := filepath.Join(dir, "chats", threadID)

	// Create floating ledger
	ledgerPath := filepath.Join(chatsDir, "conversation.jsonl")
	uncommittedText := "user: update api\n"
	if err := os.WriteFile(ledgerPath, []byte(uncommittedText), 0644); err != nil {
		t.Fatalf("failed to write floating ledger: %v", err)
	}

	// Propose candidate
	files := map[string][]byte{
		"api.go": []byte("package api\n"),
	}
	if _, err := engine.Propose(ctx, dir, threadID, proposalID, files, "test-tool"); err != nil {
		t.Fatalf("Propose failed: %v", err)
	}

	// Accept candidate
	if err := engine.Accept(ctx, dir, threadID, proposalID); err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	// VERIFY A: Safely returned to thread branch
	repo, _ := git.PlainOpen(dir)
	head, _ := repo.Head()
	if expected := "refs/heads/chat/" + threadID; head.Name().String() != expected {
		t.Errorf("Expected branch %s, got %s", expected, head.Name().String())
	}

	// VERIFY B: Ledger floated safely back across double checkout
	content, err := os.ReadFile(ledgerPath)
	if err != nil || string(content) != uncommittedText {
		t.Errorf("Uncommitted ledger was lost! Content: %s", string(content))
	}

	// VERIFY C: Proposed file PERSISTS on disk because it was successfully merged
	apiPath := filepath.Join(chatsDir, "docs", "api.go")
	if _, err := os.Stat(apiPath); os.IsNotExist(err) {
		t.Errorf("Proposed file %s was missing after acceptance", apiPath)
	}
}
