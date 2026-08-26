package gitfs_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/gitfs"
)

func setupExecAgentTestEnvironment(t *testing.T) (context.Context, string, *gitfs.GoExecEngine, string, string) {
	ctx := context.Background()
	mainDir := t.TempDir()
	engine := gitfs.NewGoExecEngine()

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

func getGitBranch(t *testing.T, dir string) string {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get git branch: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func getGitTagMessage(t *testing.T, dir string, tag string) string {
	cmd := exec.Command("git", "tag", "-l", "--format=%(contents:subject)", tag)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get git tag message: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func TestGoExecEngine_ReadCandidateDiff(t *testing.T) {
	ctx, mainDir, engine, threadID, candidateID := setupExecAgentTestEnvironment(t)

	sandboxDir, err := engine.SpawnSandbox(ctx, mainDir, threadID, candidateID)
	if err != nil {
		t.Fatalf("SpawnSandbox failed: %v", err)
	}

	testFilePath := filepath.Join(sandboxDir, "chats", threadID, "docs", "feature.go")
	os.MkdirAll(filepath.Dir(testFilePath), 0755)
	expectedContent := "package feature\n\nfunc Run() {}\n"
	os.WriteFile(testFilePath, []byte(expectedContent), 0644)

	_, err = engine.CommitSandbox(ctx, sandboxDir, "feat: add feature")
	if err != nil {
		t.Fatalf("CommitSandbox failed: %v", err)
	}

	if err := engine.SubmitSandbox(ctx, sandboxDir, mainDir, candidateID); err != nil {
		t.Fatalf("SubmitSandbox failed: %v", err)
	}

	diffStr, err := engine.ReadCandidateDiff(ctx, mainDir, threadID, candidateID)
	if err != nil {
		t.Fatalf("ReadCandidateDiff failed: %v", err)
	}

	if !strings.Contains(diffStr, "func Run() {}") {
		t.Errorf("Diff did not contain expected added text. Got:\n%s", diffStr)
	}
}

func TestGoExecEngine_AgentSandbox_Lifecycle_Reject(t *testing.T) {
	ctx, mainDir, engine, threadID, candidateID := setupExecAgentTestEnvironment(t)
	chatsDir := filepath.Join(mainDir, "chats", threadID)

	ledgerPath := filepath.Join(chatsDir, "conversation.jsonl")
	uncommittedText := "user: add authentication\n"
	os.WriteFile(ledgerPath, []byte(uncommittedText), 0644)

	sandboxDir, err := engine.SpawnSandbox(ctx, mainDir, threadID, candidateID)
	if err != nil {
		t.Fatalf("SpawnSandbox failed: %v", err)
	}

	authPath := filepath.Join(sandboxDir, "chats", threadID, "docs", "auth.go")
	os.MkdirAll(filepath.Dir(authPath), 0755)
	os.WriteFile(authPath, []byte("package auth\n"), 0644)

	_, err = engine.CommitSandbox(ctx, sandboxDir, "feat: implement auth")
	if err != nil {
		t.Fatalf("CommitSandbox failed: %v", err)
	}

	if err := engine.SubmitSandbox(ctx, sandboxDir, mainDir, candidateID); err != nil {
		t.Fatalf("SubmitSandbox failed: %v", err)
	}

	if err := engine.CloseSandbox(ctx, sandboxDir); err != nil {
		t.Fatalf("CloseSandbox failed: %v", err)
	}
	if _, err := os.Stat(sandboxDir); !os.IsNotExist(err) {
		t.Errorf("Sandbox directory was not deleted!")
	}

	if err := engine.PreviewCandidate(ctx, mainDir, threadID, candidateID); err != nil {
		t.Fatalf("PreviewCandidate failed: %v", err)
	}

	reason := "CLI manual rejection due to security flaw"
	if err := engine.Reject(ctx, mainDir, threadID, candidateID, reason); err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	if expected := "chat/" + threadID; getGitBranch(t, mainDir) != expected {
		t.Errorf("Expected branch %s, got %s", expected, getGitBranch(t, mainDir))
	}

	content, _ := os.ReadFile(ledgerPath)
	if string(content) != uncommittedText {
		t.Errorf("Uncommitted ledger was lost!")
	}

	tagName := "proposal-" + candidateID
	tagMsg := getGitTagMessage(t, mainDir, tagName)
	if tagMsg != "REJECTED: "+reason {
		t.Errorf("Tag reason mismatch. Got: %s", tagMsg)
	}
}

func TestGoExecEngine_AgentSandbox_Lifecycle_Accept(t *testing.T) {
	ctx, mainDir, engine, threadID, candidateID := setupExecAgentTestEnvironment(t)

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

	tagName := "proposal-" + candidateID
	tagMsg := getGitTagMessage(t, mainDir, tagName)

	if tagMsg != "ACCEPTED: "+reason {
		t.Errorf("Tag reason mismatch. Got: %s", tagMsg)
	}

	apiCheck := filepath.Join(mainDir, "chats", threadID, "docs", "api.go")
	if _, err := os.Stat(apiCheck); os.IsNotExist(err) {
		t.Errorf("Proposed file %s was missing after acceptance", apiCheck)
	}
}

func TestGoExecEngine_PreviewCandidate_FloatsLedger(t *testing.T) {
	ctx, mainDir, engine, threadID, candidateID := setupExecAgentTestEnvironment(t)

	ledgerPath := filepath.Join(mainDir, "chats", threadID, "conversation.jsonl")

	// 1. Create and commit the ledger so it is a TRACKED file
	os.WriteFile(ledgerPath, []byte("initial baseline\n"), 0644)
	if _, err := engine.Snapshot(ctx, mainDir, threadID, "chore: initial checkpoint"); err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	// 2. Propose a candidate
	sandboxDir, _ := engine.SpawnSandbox(ctx, mainDir, threadID, candidateID)
	testFilePath := filepath.Join(sandboxDir, "chats", threadID, "docs", "test.go")
	os.MkdirAll(filepath.Dir(testFilePath), 0755)
	os.WriteFile(testFilePath, []byte("package test\n"), 0644)

	engine.CommitSandbox(ctx, sandboxDir, "feat: candidate code")
	engine.SubmitSandbox(ctx, sandboxDir, mainDir, candidateID)
	engine.CloseSandbox(ctx, sandboxDir)

	// 3. Modify the tracked ledger (creating UNSTAGED modifications)
	modifiedText := "initial baseline\nuser: check this preview\n"
	os.WriteFile(ledgerPath, []byte(modifiedText), 0644)

	// 4. Trigger the Preview (This should safely float the unstaged modifications)
	if err := engine.PreviewCandidate(ctx, mainDir, threadID, candidateID); err != nil {
		t.Fatalf("PreviewCandidate failed to float uncommitted ledger: %v", err)
	}

	// 5. Verify the unstaged modifications survived the branch switch
	content, _ := os.ReadFile(ledgerPath)
	if string(content) != modifiedText {
		t.Errorf("Ledger content was overwritten! Expected %q, got %q", modifiedText, string(content))
	}
}
