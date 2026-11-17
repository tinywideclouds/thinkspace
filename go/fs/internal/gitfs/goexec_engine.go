package gitfs

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// GoExecEngine implements the workspace.ChatVersioner interface using the native Git CLI.
type GoExecEngine struct {
	mu             sync.Mutex
	workspaceRoot  string
	sharedCodebase bool
}

func NewGoExecEngine(workspaceRoot string, sharedCodebase bool) *GoExecEngine {
	return &GoExecEngine{
		workspaceRoot:  workspaceRoot,
		sharedCodebase: sharedCodebase,
	}
}

func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

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
		return "", fmt.Errorf("git %s failed: %w - %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}

	return strings.TrimSpace(stdout.String()), nil
}

func (c *GoExecEngine) InitChat(ctx context.Context, chatID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := runGit(ctx, c.workspaceRoot, "rev-parse", "--git-dir"); err != nil {
		if _, err := runGit(ctx, c.workspaceRoot, "init"); err != nil {
			return err
		}
		if _, err := runGit(ctx, c.workspaceRoot, "commit", "--allow-empty", "-m", "chore: initialize workspace"); err != nil {
			return err
		}
	}

	threadBranch := fmt.Sprintf("chat/%s", chatID)

	if _, err := runGit(ctx, c.workspaceRoot, "show-ref", "--verify", "--quiet", "refs/heads/"+threadBranch); err != nil {
		if _, err := runGit(ctx, c.workspaceRoot, "switch", "-c", threadBranch); err != nil {
			return err
		}
	} else {
		currentBranch, branchErr := runGit(ctx, c.workspaceRoot, "branch", "--show-current")
		if branchErr != nil {
			return branchErr
		}
		if currentBranch != threadBranch {
			if _, err := runGit(ctx, c.workspaceRoot, "switch", threadBranch); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *GoExecEngine) Snapshot(ctx context.Context, chatID string, message string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := runGit(ctx, c.workspaceRoot, "add", "--all"); err != nil {
		return "", err
	}
	if _, err := runGit(ctx, c.workspaceRoot, "commit", "--allow-empty", "-m", message); err != nil {
		return "", err
	}
	return runGit(ctx, c.workspaceRoot, "rev-parse", "HEAD")
}

func (c *GoExecEngine) SpawnCandidateSandbox(ctx context.Context, chatID string, candidateID string) (workspace.CandidateSandbox, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	sandboxDirectory, err := os.MkdirTemp("", fmt.Sprintf("sandbox-%s-*", candidateID))
	if err != nil {
		return nil, fmt.Errorf("creating temp sandbox dir: %w", err)
	}

	candidateBranch := fmt.Sprintf("candidate/%s", candidateID)
	threadBranch := fmt.Sprintf("chat/%s", chatID)

	if _, err := runGit(ctx, c.workspaceRoot, "worktree", "add", "-b", candidateBranch, sandboxDirectory, threadBranch); err != nil {
		_ = os.RemoveAll(sandboxDirectory)
		return nil, err
	}

	return &execSandbox{
		repoMu:            &c.mu,
		mainRepoDirectory: c.workspaceRoot,
		sandboxDirectory:  sandboxDirectory,
		chatDirectory:     filepath.Join(sandboxDirectory, "chats", chatID),
		candidateID:       candidateID,
		sharedCodebase:    c.sharedCodebase,
	}, nil
}

func (c *GoExecEngine) PreviewCandidate(ctx context.Context, chatID string, candidateID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	candidateBranch := fmt.Sprintf("candidate/%s", candidateID)
	_, err := runGit(ctx, c.workspaceRoot, "switch", candidateBranch)
	return err
}

func (c *GoExecEngine) ReadCandidateDiff(ctx context.Context, chatID string, candidateID string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	threadBranch := fmt.Sprintf("chat/%s", chatID)
	candidateBranch := fmt.Sprintf("candidate/%s", candidateID)

	if _, err := runGit(ctx, c.workspaceRoot, "show-ref", "--verify", "--quiet", "refs/heads/"+candidateBranch); err == nil {
		diff, err := runGit(ctx, c.workspaceRoot, "diff", threadBranch+"..."+candidateBranch)
		if err != nil {
			return "", fmt.Errorf("generating git diff: %w", err)
		}
		return diff, nil
	}

	tagName := fmt.Sprintf("proposal-%s", candidateID)
	if _, err := runGit(ctx, c.workspaceRoot, "show-ref", "--verify", "--quiet", "refs/tags/"+tagName); err == nil {
		diff, err := runGit(ctx, c.workspaceRoot, "diff", tagName+"^1.."+tagName)
		if err != nil {
			return "", fmt.Errorf("generating git diff from proposal tag %s: %w", tagName, err)
		}
		return diff, nil
	}

	return "", fmt.Errorf("candidate %s not found as active branch or proposal tag", candidateID)
}

func (c *GoExecEngine) Accept(ctx context.Context, chatID string, candidateID string, reason string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	threadBranch := fmt.Sprintf("chat/%s", chatID)
	candidateBranch := fmt.Sprintf("candidate/%s", candidateID)
	tagName := fmt.Sprintf("proposal-%s", candidateID)
	tagMsg := fmt.Sprintf("ACCEPTED: %s", reason)

	if _, err := runGit(ctx, c.workspaceRoot, "switch", threadBranch); err != nil {
		return err
	}
	if _, err := runGit(ctx, c.workspaceRoot, "merge", "--ff-only", candidateBranch); err != nil {
		return err
	}
	if _, err := runGit(ctx, c.workspaceRoot, "tag", "-a", tagName, "-m", tagMsg, candidateBranch); err != nil {
		return err
	}
	_, err := runGit(ctx, c.workspaceRoot, "branch", "-d", candidateBranch)
	return err
}

func (c *GoExecEngine) Reject(ctx context.Context, chatID string, candidateID string, reason string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	threadBranch := fmt.Sprintf("chat/%s", chatID)
	candidateBranch := fmt.Sprintf("candidate/%s", candidateID)
	tagName := fmt.Sprintf("proposal-%s", candidateID)
	tagMsg := fmt.Sprintf("REJECTED: %s", reason)

	if _, err := runGit(ctx, c.workspaceRoot, "switch", threadBranch); err != nil {
		return err
	}
	if _, err := runGit(ctx, c.workspaceRoot, "tag", "-a", tagName, "-m", tagMsg, candidateBranch); err != nil {
		return err
	}
	_, err := runGit(ctx, c.workspaceRoot, "branch", "-D", candidateBranch)
	return err
}

type execSandbox struct {
	repoMu            *sync.Mutex
	mainRepoDirectory string
	sandboxDirectory  string
	chatDirectory     string
	candidateID       string
	sharedCodebase    bool
}

func (s *execSandbox) resolveSafePath(targetPath string) (string, error) {
	baseDir := s.chatDirectory
	if s.sharedCodebase {
		baseDir = s.sandboxDirectory
	}

	cleaned := filepath.Clean(targetPath)
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("absolute path %q is not allowed in sandbox", targetPath)
	}

	slashPath := filepath.ToSlash(cleaned)
	chatID := filepath.Base(s.chatDirectory)
	slashPrefix := fmt.Sprintf("chats/%s", chatID)

	if slashPath == slashPrefix {
		slashPath = "."
	} else if strings.HasPrefix(slashPath, slashPrefix+"/") {
		slashPath = strings.TrimPrefix(slashPath, slashPrefix+"/")
	}

	fullPath := filepath.Join(baseDir, filepath.FromSlash(slashPath))
	rel, err := filepath.Rel(baseDir, fullPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes sandbox root", targetPath)
	}

	return fullPath, nil
}

func (s *execSandbox) ResolvePath(path string) string {
	fullPath, err := s.resolveSafePath(path)
	if err != nil {
		if s.sharedCodebase {
			return s.sandboxDirectory
		}
		return s.chatDirectory
	}
	return fullPath
}

func (s *execSandbox) WriteFile(ctx context.Context, path string, data []byte) error {
	fullPath, err := s.resolveSafePath(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, data, 0644)
}

func (s *execSandbox) ReadFile(ctx context.Context, path string) ([]byte, error) {
	fullPath, err := s.resolveSafePath(path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(fullPath)
}

func (s *execSandbox) ExecuteCommand(ctx context.Context, command string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	if s.sharedCodebase {
		cmd.Dir = s.sandboxDirectory
	} else {
		cmd.Dir = s.chatDirectory
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("exec %s failed: %w - %s", command, err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (s *execSandbox) ApplyDraft(ctx context.Context, message string) error {
	if _, err := runGit(ctx, s.sandboxDirectory, "add", "--all"); err != nil {
		return err
	}
	_, err := runGit(ctx, s.sandboxDirectory, "commit", "--allow-empty", "-m", message)
	return err
}

func (s *execSandbox) DeliverForReview(ctx context.Context) error {
	return nil
}

func (s *execSandbox) TearDown(ctx context.Context) error {
	if s.repoMu != nil {
		s.repoMu.Lock()
		defer s.repoMu.Unlock()
	}
	_, err := runGit(ctx, s.mainRepoDirectory, "worktree", "remove", "--force", s.sandboxDirectory)
	return err
}
