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

func setupAgentTestEnvironment(t *testing.T) (context.Context, string, *gitfs.GoGitEngine, string, string) {
	ctx := context.Background()
	mainDir := t.TempDir()
	engine := gitfs.NewGoGitEngine()

	threadID := "test-thread"
	candidateID := "test-candidate"

	if err := engine.InitThread(ctx, mainDir, threadID); err != nil {
		t.Fatalf("InitThread failed: %v", err)
	}

	chatsDir := filepath.Join(mainDir, "chats", threadID)
	if err := os.MkdirAll(chatsDir, 0755); err != nil {
		t.Fatalf("failed to create chats dir: %v", err)
	}

	return ctx, mainDir, engine, threadID, candidateID
}

func TestGoGitEngine_AgentSandbox_Lifecycle_Reject(t *testing.T) {
	ctx, mainDir, engine, threadID, candidateID := setupAgentTestEnvironment(t)
	chatsDir := filepath.Join(mainDir, "chats", threadID)

	ledgerPath := filepath.Join(chatsDir, "conversation.jsonl")
	uncommittedText := "user: add authentication\n"
	os.WriteFile(ledgerPath, []byte(uncommittedText), 0644)

	// 1. SPAWN
	sandboxDir, err := engine.SpawnSandbox(ctx, mainDir, threadID, candidateID)
	if err != nil {
		t.Fatalf("SpawnSandbox failed: %v", err)
	}

	// 2. COMMIT (Agent Work)
	authPath := filepath.Join(sandboxDir, "chats", threadID, "docs", "auth.go")
	os.MkdirAll(filepath.Dir(authPath), 0755)
	os.WriteFile(authPath, []byte("package auth\n"), 0644)

	_, err = engine.CommitSandbox(ctx, sandboxDir, "feat: implement auth")
	if err != nil {
		t.Fatalf("CommitSandbox failed: %v", err)
	}

	// 3. SUBMIT
	if err := engine.SubmitSandbox(ctx, sandboxDir, mainDir, candidateID); err != nil {
		t.Fatalf("SubmitSandbox failed: %v", err)
	}

	// 4. CLOSE
	if err := engine.CloseSandbox(ctx, sandboxDir); err != nil {
		t.Fatalf("CloseSandbox failed: %v", err)
	}
	if _, err := os.Stat(sandboxDir); !os.IsNotExist(err) {
		t.Errorf("Sandbox directory was not deleted!")
	}

	// 5. PREVIEW
	if err := engine.PreviewCandidate(ctx, mainDir, threadID, candidateID); err != nil {
		t.Fatalf("PreviewCandidate failed: %v", err)
	}

	// 6. REJECT
	reason := "CLI manual rejection due to security flaw"
	if err := engine.Reject(ctx, mainDir, threadID, candidateID, reason); err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	// VERIFY A: Returned to thread branch cleanly
	repo, _ := git.PlainOpen(mainDir)
	head, _ := repo.Head()
	if expected := "refs/heads/chat/" + threadID; head.Name().String() != expected {
		t.Errorf("Expected branch %s, got %s", expected, head.Name().String())
	}

	// VERIFY B: Ledger floated safely back
	content, _ := os.ReadFile(ledgerPath)
	if string(content) != uncommittedText {
		t.Errorf("Uncommitted ledger was lost!")
	}

	// VERIFY C: Annotated tag captures reason permanently
	tagRefName := plumbing.ReferenceName("refs/tags/proposal-" + candidateID)
	ref, err := repo.Reference(tagRefName, true)
	if err != nil {
		t.Fatalf("Proposal tag was destroyed: %v", err)
	}

	tagObj, _ := repo.TagObject(ref.Hash())
	if tagObj.Message != "REJECTED: "+reason {
		t.Errorf("Tag reason mismatch. Got: %s", tagObj.Message)
	}
}

func TestGoGitEngine_AgentSandbox_Lifecycle_Accept(t *testing.T) {
	ctx, mainDir, engine, threadID, candidateID := setupAgentTestEnvironment(t)

	sandboxDir, _ := engine.SpawnSandbox(ctx, mainDir, threadID, candidateID)

	apiPath := filepath.Join(sandboxDir, "chats", threadID, "docs", "api.go")
	os.MkdirAll(filepath.Dir(apiPath), 0755)
	os.WriteFile(apiPath, []byte("package api\n"), 0644)

	engine.CommitSandbox(ctx, sandboxDir, "feat: basic api")
	engine.SubmitSandbox(ctx, sandboxDir, mainDir, candidateID)
	engine.CloseSandbox(ctx, sandboxDir)
	engine.PreviewCandidate(ctx, mainDir, threadID, candidateID)

	reason := "Looks perfect"
	if err := engine.Accept(ctx, mainDir, threadID, candidateID, reason); err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	repo, _ := git.PlainOpen(mainDir)
	tagRefName := plumbing.ReferenceName("refs/tags/proposal-" + candidateID)
	ref, _ := repo.Reference(tagRefName, true)
	tagObj, _ := repo.TagObject(ref.Hash())

	if tagObj.Message != "ACCEPTED: "+reason {
		t.Errorf("Tag reason mismatch. Got: %s", tagObj.Message)
	}

	apiCheck := filepath.Join(mainDir, "chats", threadID, "docs", "api.go")
	if _, err := os.Stat(apiCheck); os.IsNotExist(err) {
		t.Errorf("Proposed file %s was missing after acceptance", apiCheck)
	}
}
