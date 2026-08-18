package gitfs

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GoExecEngine implements the workspace.StateEngine interface by wrapping the standard git CLI.
type GoExecEngine struct{}

func NewGoExecEngine() *GoExecEngine {
	return &GoExecEngine{}
}

// runGit is a helper to execute git commands with the correct environment and directory.
func (e *GoExecEngine) runGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	// Ensure Git doesn't block waiting for identity configuration
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Workspace Engine",
		"GIT_AUTHOR_EMAIL=engine@local.workspace",
		"GIT_COMMITTER_NAME=Workspace Engine",
		"GIT_COMMITTER_EMAIL=engine@local.workspace",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s failed: %w - %s", strings.Join(args, " "), err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

func (e *GoExecEngine) InitThread(ctx context.Context, mainDir string, threadID string) error {
	if _, err := e.runGit(ctx, mainDir, "rev-parse", "--git-dir"); err != nil {
		if _, err := e.runGit(ctx, mainDir, "init"); err != nil {
			return err
		}
		if _, err := e.runGit(ctx, mainDir, "commit", "--allow-empty", "-m", "chore: initialize workspace"); err != nil {
			return err
		}
	}

	threadBranch := fmt.Sprintf("chat/%s", threadID)

	// Check if branch exists
	if _, err := e.runGit(ctx, mainDir, "show-ref", "--verify", "--quiet", "refs/heads/"+threadBranch); err != nil {
		// Create and switch
		if _, err := e.runGit(ctx, mainDir, "switch", "-c", threadBranch); err != nil {
			return err
		}
	} else {
		// Just switch
		if _, err := e.runGit(ctx, mainDir, "switch", threadBranch); err != nil {
			return err
		}
	}

	return nil
}

func (e *GoExecEngine) Snapshot(ctx context.Context, mainDir string, threadID string, message string) (string, error) {
	if _, err := e.runGit(ctx, mainDir, "add", "--all"); err != nil {
		return "", err
	}
	if _, err := e.runGit(ctx, mainDir, "commit", "--allow-empty", "-m", message); err != nil {
		return "", err
	}
	return e.runGit(ctx, mainDir, "rev-parse", "HEAD")
}

// --- Agentic Sandbox Management ---

func (e *GoExecEngine) SpawnSandbox(ctx context.Context, mainDir string, threadID string, candidateID string) (string, error) {
	// Generate a unique path in the OS temp directory
	sandboxDir := filepath.Join(os.TempDir(), fmt.Sprintf("sandbox-%s-%d", candidateID, os.Getpid()))

	candidateBranch := fmt.Sprintf("candidate/%s", candidateID)
	threadBranch := fmt.Sprintf("chat/%s", threadID)

	// git worktree add creates the directory, branches off the threadBranch, and checks it out.
	if _, err := e.runGit(ctx, mainDir, "worktree", "add", "-b", candidateBranch, sandboxDir, threadBranch); err != nil {
		return "", err
	}

	return sandboxDir, nil
}

func (e *GoExecEngine) CommitSandbox(ctx context.Context, sandboxDir string, message string) (string, error) {
	if _, err := e.runGit(ctx, sandboxDir, "add", "--all"); err != nil {
		return "", err
	}
	if _, err := e.runGit(ctx, sandboxDir, "commit", "--allow-empty", "-m", message); err != nil {
		return "", err
	}
	return e.runGit(ctx, sandboxDir, "rev-parse", "HEAD")
}

func (e *GoExecEngine) SubmitSandbox(ctx context.Context, sandboxDir string, mainDir string, candidateID string) error {
	// No-op for CLI Worktrees.
	// The sandbox is physically linked to the main repository's database.
	// Any commits made in the sandbox are instantly available in the main repository.
	return nil
}

func (e *GoExecEngine) CloseSandbox(ctx context.Context, sandboxDir string) error {
	// We run the remove command from the sandbox dir's parent to be safe, or mainDir.
	// We use --force to discard any uncommitted files left in the sandbox.
	_, err := e.runGit(ctx, filepath.Dir(sandboxDir), "worktree", "remove", "--force", sandboxDir)
	return err
}

// --- Main Session / User Review ---

func (e *GoExecEngine) PreviewCandidate(ctx context.Context, mainDir string, threadID string, candidateID string) error {
	candidateBranch := fmt.Sprintf("candidate/%s", candidateID)
	_, err := e.runGit(ctx, mainDir, "switch", candidateBranch)
	return err
}

func (e *GoExecEngine) Accept(ctx context.Context, mainDir string, threadID string, candidateID string, reason string) error {
	threadBranch := fmt.Sprintf("chat/%s", threadID)
	candidateBranch := fmt.Sprintf("candidate/%s", candidateID)
	tagName := fmt.Sprintf("proposal-%s", candidateID)
	tagMsg := fmt.Sprintf("ACCEPTED: %s", reason)

	if _, err := e.runGit(ctx, mainDir, "switch", threadBranch); err != nil {
		return err
	}

	// Fast-forward merge
	if _, err := e.runGit(ctx, mainDir, "merge", "--ff-only", candidateBranch); err != nil {
		return err
	}

	// Create annotated tag
	if _, err := e.runGit(ctx, mainDir, "tag", "-a", tagName, "-m", tagMsg, candidateBranch); err != nil {
		return err
	}

	// Delete branch
	_, err := e.runGit(ctx, mainDir, "branch", "-d", candidateBranch)
	return err
}

func (e *GoExecEngine) Reject(ctx context.Context, mainDir string, threadID string, candidateID string, reason string) error {
	threadBranch := fmt.Sprintf("chat/%s", threadID)
	candidateBranch := fmt.Sprintf("candidate/%s", candidateID)
	tagName := fmt.Sprintf("proposal-%s", candidateID)
	tagMsg := fmt.Sprintf("REJECTED: %s", reason)

	if _, err := e.runGit(ctx, mainDir, "switch", threadBranch); err != nil {
		return err
	}

	// Create annotated tag
	if _, err := e.runGit(ctx, mainDir, "tag", "-a", tagName, "-m", tagMsg, candidateBranch); err != nil {
		return err
	}

	// Force delete branch since it was not merged
	_, err := e.runGit(ctx, mainDir, "branch", "-D", candidateBranch)
	return err
}
