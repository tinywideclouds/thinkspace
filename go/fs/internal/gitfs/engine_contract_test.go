package gitfs_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/tinywideclouds.com/thinkspace/internal/gitfs"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type EngineFactory func(dir string, sharedCodebase bool) workspace.ChatEngine

func getGitBranch(t *testing.T, dir string) string {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get git branch: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func setupTestEnvironment(t *testing.T, factory EngineFactory, sharedCodebase bool) (context.Context, string, workspace.ChatEngine, string, string) {
	ctx := context.Background()
	mainDir := t.TempDir()
	engine := factory(mainDir, sharedCodebase)

	threadID := "test-thread"
	candidateID := "test-candidate"

	if err := engine.InitChat(ctx, threadID); err != nil {
		t.Fatalf("InitChat failed: %v", err)
	}

	chatsDir := filepath.Join(mainDir, "chats", threadID)
	if err := os.MkdirAll(chatsDir, 0755); err != nil {
		t.Fatalf("failed to create chats dir: %v", err)
	}

	return ctx, mainDir, engine, threadID, candidateID
}

func TestChatEngines(t *testing.T) {
	engines := map[string]EngineFactory{
		"GoGit":  func(dir string, shared bool) workspace.ChatEngine { return gitfs.NewGoGitChat(dir, shared) },
		"GoExec": func(dir string, shared bool) workspace.ChatEngine { return gitfs.NewGoExecChat(dir, shared) },
	}

	for name, factory := range engines {
		t.Run(name, func(t *testing.T) {
			// We test in Jailed mode (SharedCodebase=false) to ensure identical behavior to the legacy tests
			// and verify that the file path normalizer works correctly.
			t.Run("ReadCandidateDiff", func(t *testing.T) { testReadCandidateDiff(t, factory, false) })
			t.Run("Lifecycle_Reject", func(t *testing.T) { testAgentSandboxLifecycleReject(t, factory, false) })
			t.Run("Lifecycle_Accept", func(t *testing.T) { testAgentSandboxLifecycleAccept(t, factory, false) })
			t.Run("PreviewCandidate_FloatsLedger", func(t *testing.T) { testPreviewCandidateFloatsLedger(t, factory, false) })

			// Additional matrix run for SharedCodebase=true on Accept to ensure files route to the root
			t.Run("Lifecycle_Accept_SharedCodebase", func(t *testing.T) { testAgentSandboxLifecycleAccept(t, factory, true) })
		})
	}
}

func testReadCandidateDiff(t *testing.T, factory EngineFactory, sharedCodebase bool) {
	ctx, _, engine, threadID, candidateID := setupTestEnvironment(t, factory, sharedCodebase)

	sandbox, err := engine.SpawnCandidateSandbox(ctx, threadID, candidateID)
	if err != nil {
		t.Fatalf("SpawnCandidateSandbox failed: %v", err)
	}

	testFilePath := filepath.Join("chats", threadID, "docs", "feature.go")
	if err := sandbox.WriteFile(ctx, testFilePath, []byte("package feature\n\nfunc Run() {}\n")); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if err := sandbox.ApplyDraft(ctx, "feat: add feature"); err != nil {
		t.Fatalf("ApplyDraft failed: %v", err)
	}

	if err := sandbox.DeliverForReview(ctx); err != nil {
		t.Fatalf("DeliverForReview failed: %v", err)
	}

	diffStr, err := engine.ReadCandidateDiff(ctx, threadID, candidateID)
	if err != nil {
		t.Fatalf("ReadCandidateDiff failed: %v", err)
	}

	if !strings.Contains(diffStr, "func Run() {}") {
		t.Errorf("Diff did not contain expected added text. Got:\n%s", diffStr)
	}
}

func testAgentSandboxLifecycleReject(t *testing.T, factory EngineFactory, sharedCodebase bool) {
	ctx, mainDir, engine, threadID, candidateID := setupTestEnvironment(t, factory, sharedCodebase)
	chatsDir := filepath.Join(mainDir, "chats", threadID)

	ledgerPath := filepath.Join(chatsDir, "conversation.jsonl")
	uncommittedText := "user: add authentication\n"
	os.WriteFile(ledgerPath, []byte(uncommittedText), 0644)

	sandbox, err := engine.SpawnCandidateSandbox(ctx, threadID, candidateID)
	if err != nil {
		t.Fatalf("SpawnCandidateSandbox failed: %v", err)
	}

	authPath := filepath.Join("chats", threadID, "docs", "auth.go")
	sandbox.WriteFile(ctx, authPath, []byte("package auth\n"))
	sandbox.ApplyDraft(ctx, "feat: implement auth")
	sandbox.DeliverForReview(ctx)
	sandbox.TearDown(ctx)

	if err := engine.PreviewCandidate(ctx, threadID, candidateID); err != nil {
		t.Fatalf("PreviewCandidate failed: %v", err)
	}

	reason := "CLI manual rejection due to security flaw"
	if err := engine.Reject(ctx, threadID, candidateID, reason); err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	if expected := "chat/" + threadID; getGitBranch(t, mainDir) != expected {
		t.Errorf("Expected branch %s, got %s", expected, getGitBranch(t, mainDir))
	}

	content, _ := os.ReadFile(ledgerPath)
	if string(content) != uncommittedText {
		t.Errorf("Uncommitted ledger was lost!")
	}

	repo, _ := git.PlainOpen(mainDir)
	tagRefName := plumbing.ReferenceName("refs/tags/proposal-" + candidateID)
	ref, err := repo.Reference(tagRefName, true)
	if err != nil {
		t.Fatalf("Proposal tag was destroyed: %v", err)
	}

	tagObj, _ := repo.TagObject(ref.Hash())
	if strings.TrimSpace(tagObj.Message) != "REJECTED: "+reason {
		t.Errorf("Tag reason mismatch. Got: %s", tagObj.Message)
	}
}

func testAgentSandboxLifecycleAccept(t *testing.T, factory EngineFactory, sharedCodebase bool) {
	ctx, mainDir, engine, threadID, candidateID := setupTestEnvironment(t, factory, sharedCodebase)

	sandbox, _ := engine.SpawnCandidateSandbox(ctx, threadID, candidateID)

	apiPath := filepath.Join("chats", threadID, "docs", "api.go")
	sandbox.WriteFile(ctx, apiPath, []byte("package api\n"))
	sandbox.ApplyDraft(ctx, "feat: basic api")
	sandbox.DeliverForReview(ctx)
	sandbox.TearDown(ctx)

	engine.PreviewCandidate(ctx, threadID, candidateID)

	reason := "Looks perfect"
	if err := engine.Accept(ctx, threadID, candidateID, reason); err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	repo, _ := git.PlainOpen(mainDir)
	tagRefName := plumbing.ReferenceName("refs/tags/proposal-" + candidateID)
	ref, _ := repo.Reference(tagRefName, true)
	tagObj, _ := repo.TagObject(ref.Hash())

	if strings.TrimSpace(tagObj.Message) != "ACCEPTED: "+reason {
		t.Errorf("Tag reason mismatch. Got: %s", tagObj.Message)
	}

	// Dynamic path check based on the execution mode
	var apiCheck string
	if sharedCodebase {
		apiCheck = filepath.Join(mainDir, "docs", "api.go")
	} else {
		apiCheck = filepath.Join(mainDir, "chats", threadID, "docs", "api.go")
	}

	if _, err := os.Stat(apiCheck); os.IsNotExist(err) {
		t.Errorf("Proposed file %s was missing after acceptance", apiCheck)
	}
}

func testPreviewCandidateFloatsLedger(t *testing.T, factory EngineFactory, sharedCodebase bool) {
	ctx, mainDir, engine, threadID, candidateID := setupTestEnvironment(t, factory, sharedCodebase)

	ledgerPath := filepath.Join(mainDir, "chats", threadID, "conversation.jsonl")

	os.WriteFile(ledgerPath, []byte("initial baseline\n"), 0644)
	if _, err := engine.Snapshot(ctx, threadID, "chore: initial checkpoint"); err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	sandbox, _ := engine.SpawnCandidateSandbox(ctx, threadID, candidateID)
	testPath := filepath.Join("chats", threadID, "docs", "test.go")
	sandbox.WriteFile(ctx, testPath, []byte("package test\n"))
	sandbox.ApplyDraft(ctx, "feat: candidate code")
	sandbox.DeliverForReview(ctx)
	sandbox.TearDown(ctx)

	modifiedText := "initial baseline\nuser: check this preview\n"
	os.WriteFile(ledgerPath, []byte(modifiedText), 0644)

	if err := engine.PreviewCandidate(ctx, threadID, candidateID); err != nil {
		t.Fatalf("PreviewCandidate failed to float uncommitted ledger: %v", err)
	}

	content, _ := os.ReadFile(ledgerPath)
	if string(content) != modifiedText {
		t.Errorf("Ledger content was overwritten! Expected %q, got %q", modifiedText, string(content))
	}
}
