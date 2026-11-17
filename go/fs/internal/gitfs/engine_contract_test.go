package gitfs_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/tinywideclouds.com/thinkspace/internal/gitfs"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type EngineFactory func(dir string, sharedCodebase bool) workspace.ChatVersioner

func getGitBranch(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get git branch: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func setupTestEnvironment(t *testing.T, factory EngineFactory, sharedCodebase bool) (context.Context, string, workspace.ChatVersioner, string, string) {
	t.Helper()
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
		"GoGitEngine":  func(dir string, shared bool) workspace.ChatVersioner { return gitfs.NewGoGitEngine(dir, shared) },
		"GoExecEngine": func(dir string, shared bool) workspace.ChatVersioner { return gitfs.NewGoExecEngine(dir, shared) },
	}

	for name, factory := range engines {
		t.Run(name, func(t *testing.T) {
			t.Run("ReadCandidateDiff", func(t *testing.T) { testReadCandidateDiff(t, factory, false) })
			t.Run("ReadCandidateDiff_AfterResolution", func(t *testing.T) { testReadCandidateDiffAfterResolution(t, factory, false) })
			t.Run("Lifecycle_Reject", func(t *testing.T) { testAgentSandboxLifecycleReject(t, factory, false) })
			t.Run("Lifecycle_Accept", func(t *testing.T) { testAgentSandboxLifecycleAccept(t, factory, false) })
			t.Run("PreviewCandidate_FloatsLedger", func(t *testing.T) { testPreviewCandidateFloatsLedger(t, factory, false) })
			t.Run("Lifecycle_Accept_SharedCodebase", func(t *testing.T) { testAgentSandboxLifecycleAccept(t, factory, true) })
			t.Run("Sandbox_PathTraversalRejected", func(t *testing.T) { testSandboxPathTraversalRejected(t, factory) })
			t.Run("Sandbox_ConcurrentSpawnAndTearDown", func(t *testing.T) { testConcurrentSpawnAndTearDown(t, factory) })
		})
	}
}

func testReadCandidateDiff(t *testing.T, factory EngineFactory, sharedCodebase bool) {
	ctx, _, engine, threadID, candidateID := setupTestEnvironment(t, factory, sharedCodebase)

	sandbox, err := engine.SpawnCandidateSandbox(ctx, threadID, candidateID)
	if err != nil {
		t.Fatalf("SpawnCandidateSandbox failed: %v", err)
	}
	defer func() {
		if tdErr := sandbox.TearDown(ctx); tdErr != nil {
			t.Errorf("TearDown failed: %v", tdErr)
		}
	}()

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

	if _, missingErr := engine.ReadCandidateDiff(ctx, threadID, "non-existent-candidate"); missingErr == nil {
		t.Errorf("expected error when reading diff for non-existent candidate")
	}
}

func testReadCandidateDiffAfterResolution(t *testing.T, factory EngineFactory, sharedCodebase bool) {
	ctx, _, engine, threadID, _ := setupTestEnvironment(t, factory, sharedCodebase)

	rejectedID := "cand-rejected-diff"
	sb1, err := engine.SpawnCandidateSandbox(ctx, threadID, rejectedID)
	if err != nil {
		t.Fatalf("SpawnCandidateSandbox failed: %v", err)
	}
	if err := sb1.WriteFile(ctx, filepath.Join("chats", threadID, "docs", "rejected.go"), []byte("package rejected\n\nfunc RejectedFn() {}\n")); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := sb1.ApplyDraft(ctx, "feat: rejected"); err != nil {
		t.Fatalf("ApplyDraft failed: %v", err)
	}
	if err := sb1.DeliverForReview(ctx); err != nil {
		t.Fatalf("DeliverForReview failed: %v", err)
	}
	if err := sb1.TearDown(ctx); err != nil {
		t.Fatalf("TearDown failed: %v", err)
	}

	if err := engine.Reject(ctx, threadID, rejectedID, "superseded"); err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	rejectedDiff, err := engine.ReadCandidateDiff(ctx, threadID, rejectedID)
	if err != nil {
		t.Fatalf("ReadCandidateDiff after Reject failed: %v", err)
	}
	if !strings.Contains(rejectedDiff, "func RejectedFn() {}") {
		t.Errorf("Expected diff from proposal-%s tag to contain RejectedFn, got:\n%s", rejectedID, rejectedDiff)
	}

	acceptedID := "cand-accepted-diff"
	sb2, err := engine.SpawnCandidateSandbox(ctx, threadID, acceptedID)
	if err != nil {
		t.Fatalf("SpawnCandidateSandbox failed: %v", err)
	}
	if err := sb2.WriteFile(ctx, filepath.Join("chats", threadID, "docs", "accepted.go"), []byte("package accepted\n\nfunc AcceptedFn() {}\n")); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := sb2.ApplyDraft(ctx, "feat: accepted"); err != nil {
		t.Fatalf("ApplyDraft failed: %v", err)
	}
	if err := sb2.DeliverForReview(ctx); err != nil {
		t.Fatalf("DeliverForReview failed: %v", err)
	}
	if err := sb2.TearDown(ctx); err != nil {
		t.Fatalf("TearDown failed: %v", err)
	}

	if err := engine.Accept(ctx, threadID, acceptedID, "ship it"); err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	acceptedDiff, err := engine.ReadCandidateDiff(ctx, threadID, acceptedID)
	if err != nil {
		t.Fatalf("ReadCandidateDiff after Accept failed: %v", err)
	}
	if !strings.Contains(acceptedDiff, "func AcceptedFn() {}") {
		t.Errorf("Expected diff from proposal-%s tag to contain AcceptedFn, got:\n%s", acceptedID, acceptedDiff)
	}
}

func testAgentSandboxLifecycleReject(t *testing.T, factory EngineFactory, sharedCodebase bool) {
	ctx, mainDir, engine, threadID, candidateID := setupTestEnvironment(t, factory, sharedCodebase)
	chatsDir := filepath.Join(mainDir, "chats", threadID)

	ledgerPath := filepath.Join(chatsDir, "conversation.jsonl")
	uncommittedText := "user: add authentication\n"
	if err := os.WriteFile(ledgerPath, []byte(uncommittedText), 0644); err != nil {
		t.Fatalf("failed writing ledger file: %v", err)
	}

	sandbox, err := engine.SpawnCandidateSandbox(ctx, threadID, candidateID)
	if err != nil {
		t.Fatalf("SpawnCandidateSandbox failed: %v", err)
	}

	authPath := filepath.Join("chats", threadID, "docs", "auth.go")
	if err := sandbox.WriteFile(ctx, authPath, []byte("package auth\n")); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := sandbox.ApplyDraft(ctx, "feat: implement auth"); err != nil {
		t.Fatalf("ApplyDraft failed: %v", err)
	}
	if err := sandbox.DeliverForReview(ctx); err != nil {
		t.Fatalf("DeliverForReview failed: %v", err)
	}
	if err := sandbox.TearDown(ctx); err != nil {
		t.Fatalf("TearDown failed: %v", err)
	}

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

	content, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("failed reading ledger file: %v", err)
	}
	if string(content) != uncommittedText {
		t.Errorf("Uncommitted ledger was lost!")
	}

	repo, err := git.PlainOpen(mainDir)
	if err != nil {
		t.Fatalf("PlainOpen failed: %v", err)
	}
	tagRefName := plumbing.ReferenceName("refs/tags/proposal-" + candidateID)
	ref, err := repo.Reference(tagRefName, true)
	if err != nil {
		t.Fatalf("Proposal tag was destroyed: %v", err)
	}

	tagObj, err := repo.TagObject(ref.Hash())
	if err != nil {
		t.Fatalf("TagObject failed: %v", err)
	}
	if strings.TrimSpace(tagObj.Message) != "REJECTED: "+reason {
		t.Errorf("Tag reason mismatch. Got: %s", tagObj.Message)
	}
}

func testAgentSandboxLifecycleAccept(t *testing.T, factory EngineFactory, sharedCodebase bool) {
	ctx, mainDir, engine, threadID, candidateID := setupTestEnvironment(t, factory, sharedCodebase)

	sandbox, err := engine.SpawnCandidateSandbox(ctx, threadID, candidateID)
	if err != nil {
		t.Fatalf("SpawnCandidateSandbox failed: %v", err)
	}

	apiPath := filepath.Join("chats", threadID, "docs", "api.go")
	if err := sandbox.WriteFile(ctx, apiPath, []byte("package api\n")); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := sandbox.ApplyDraft(ctx, "feat: basic api"); err != nil {
		t.Fatalf("ApplyDraft failed: %v", err)
	}
	if err := sandbox.DeliverForReview(ctx); err != nil {
		t.Fatalf("DeliverForReview failed: %v", err)
	}
	if err := sandbox.TearDown(ctx); err != nil {
		t.Fatalf("TearDown failed: %v", err)
	}

	if err := engine.PreviewCandidate(ctx, threadID, candidateID); err != nil {
		t.Fatalf("PreviewCandidate failed: %v", err)
	}

	reason := "Looks perfect"
	if err := engine.Accept(ctx, threadID, candidateID, reason); err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	repo, err := git.PlainOpen(mainDir)
	if err != nil {
		t.Fatalf("PlainOpen failed: %v", err)
	}
	tagRefName := plumbing.ReferenceName("refs/tags/proposal-" + candidateID)
	ref, err := repo.Reference(tagRefName, true)
	if err != nil {
		t.Fatalf("Reference failed: %v", err)
	}
	tagObj, err := repo.TagObject(ref.Hash())
	if err != nil {
		t.Fatalf("TagObject failed: %v", err)
	}

	if strings.TrimSpace(tagObj.Message) != "ACCEPTED: "+reason {
		t.Errorf("Tag reason mismatch. Got: %s", tagObj.Message)
	}

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

	if err := os.WriteFile(ledgerPath, []byte("initial baseline\n"), 0644); err != nil {
		t.Fatalf("failed writing initial ledger: %v", err)
	}
	if _, err := engine.Snapshot(ctx, threadID, "chore: initial checkpoint"); err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	sandbox, err := engine.SpawnCandidateSandbox(ctx, threadID, candidateID)
	if err != nil {
		t.Fatalf("SpawnCandidateSandbox failed: %v", err)
	}
	testPath := filepath.Join("chats", threadID, "docs", "test.go")
	if err := sandbox.WriteFile(ctx, testPath, []byte("package test\n")); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := sandbox.ApplyDraft(ctx, "feat: candidate code"); err != nil {
		t.Fatalf("ApplyDraft failed: %v", err)
	}
	if err := sandbox.DeliverForReview(ctx); err != nil {
		t.Fatalf("DeliverForReview failed: %v", err)
	}
	if err := sandbox.TearDown(ctx); err != nil {
		t.Fatalf("TearDown failed: %v", err)
	}

	modifiedText := "initial baseline\nuser: check this preview\n"
	if err := os.WriteFile(ledgerPath, []byte(modifiedText), 0644); err != nil {
		t.Fatalf("failed writing modified ledger: %v", err)
	}

	if err := engine.PreviewCandidate(ctx, threadID, candidateID); err != nil {
		t.Fatalf("PreviewCandidate failed to float uncommitted ledger: %v", err)
	}

	content, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("failed reading ledger file: %v", err)
	}
	if string(content) != modifiedText {
		t.Errorf("Ledger content was overwritten! Expected %q, got %q", modifiedText, string(content))
	}
}

func testSandboxPathTraversalRejected(t *testing.T, factory EngineFactory) {
	ctx, _, engine, threadID, candidateID := setupTestEnvironment(t, factory, true)

	sandbox, err := engine.SpawnCandidateSandbox(ctx, threadID, candidateID)
	if err != nil {
		t.Fatalf("SpawnCandidateSandbox failed: %v", err)
	}
	defer func() { _ = sandbox.TearDown(ctx) }()

	if err := sandbox.WriteFile(ctx, "../../outside.txt", []byte("evil")); err == nil {
		t.Errorf("expected WriteFile to reject path traversal")
	}
	if _, err := sandbox.ReadFile(ctx, "../../outside.txt"); err == nil {
		t.Errorf("expected ReadFile to reject path traversal")
	}
}

func testConcurrentSpawnAndTearDown(t *testing.T, factory EngineFactory) {
	ctx, _, engine, threadID, _ := setupTestEnvironment(t, factory, true)

	const numAgents = 4
	var wg sync.WaitGroup
	errs := make(chan error, numAgents)

	for i := 0; i < numAgents; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			candID := fmt.Sprintf("concurrent-cand-%d", idx)
			sb, err := engine.SpawnCandidateSandbox(ctx, threadID, candID)
			if err != nil {
				errs <- fmt.Errorf("spawn %s: %w", candID, err)
				return
			}
			if err := sb.WriteFile(ctx, fmt.Sprintf("src/agent_%d.go", idx), []byte("package main\n")); err != nil {
				_ = sb.TearDown(ctx)
				errs <- fmt.Errorf("write %s: %w", candID, err)
				return
			}
			if err := sb.ApplyDraft(ctx, "feat: concurrent"); err != nil {
				_ = sb.TearDown(ctx)
				errs <- fmt.Errorf("draft %s: %w", candID, err)
				return
			}
			if err := sb.DeliverForReview(ctx); err != nil {
				_ = sb.TearDown(ctx)
				errs <- fmt.Errorf("deliver %s: %w", candID, err)
				return
			}
			if err := sb.TearDown(ctx); err != nil {
				errs <- fmt.Errorf("teardown %s: %w", candID, err)
			}
		}(i)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent sandbox operation failed: %v", err)
	}
}
