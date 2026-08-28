package gitfs

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// GoGitChat implements the workspace.ChatEngine interface using the go-git library.
type GoGitChat struct {
	workspaceRoot  string
	sharedCodebase bool
}

func NewGoGitChat(workspaceRoot string, sharedCodebase bool) *GoGitChat {
	return &GoGitChat{
		workspaceRoot:  workspaceRoot,
		sharedCodebase: sharedCodebase,
	}
}

// withFloatingLedger is a robust wrapper that fulfills the domain requirement
// of floating uncommitted changes (like the conversation ledger). It explicitly
// stashes them, forces a clean Git transition, and restores them to disk.
func (c *GoGitChat) withFloatingLedger(worktree *git.Worktree, action func() error) error {
	status, err := worktree.Status()
	if err != nil {
		return err
	}

	// 1. Stash any uncommitted or untracked files in memory
	stash := make(map[string][]byte)
	for path, fileStatus := range status {
		if fileStatus.Worktree == git.Modified || fileStatus.Worktree == git.Untracked {
			fullPath := filepath.Join(c.workspaceRoot, path)
			data, err := os.ReadFile(fullPath)
			if err == nil {
				stash[path] = data
			}
		}
	}

	// 2. Execute the git action (using Force: true to guarantee clean extraction)
	if err := action(); err != nil {
		return err
	}

	// 3. Restore the uncommitted files safely to disk
	for path, data := range stash {
		fullPath := filepath.Join(c.workspaceRoot, path)
		_ = os.MkdirAll(filepath.Dir(fullPath), 0755)
		_ = os.WriteFile(fullPath, data, 0644)
	}

	return nil
}

func (c *GoGitChat) InitChat(ctx context.Context, chatID string) error {
	repository, err := git.PlainInit(c.workspaceRoot, false)
	if err != nil && err != git.ErrRepositoryAlreadyExists {
		return fmt.Errorf("initializing native git repository: %w", err)
	}

	if err == nil {
		headReference := plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.ReferenceName("refs/heads/main"))
		if err := repository.Storer.SetReference(headReference); err != nil {
			return fmt.Errorf("setting main branch reference: %w", err)
		}

		worktree, err := repository.Worktree()
		if err != nil {
			return fmt.Errorf("getting worktree for initial commit: %w", err)
		}

		_, err = worktree.Commit("chore: initialize workspace", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Workspace Engine",
				Email: "engine@local.workspace",
				When:  time.Now().UTC(),
			},
			AllowEmptyCommits: true,
		})
		if err != nil {
			return fmt.Errorf("creating initial commit: %w", err)
		}
	} else {
		repository, err = git.PlainOpen(c.workspaceRoot)
		if err != nil {
			return fmt.Errorf("opening repository: %w", err)
		}
	}

	threadBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("chat/%s", chatID))
	headReference, err := repository.Head()
	if err != nil {
		return fmt.Errorf("resolving HEAD: %w", err)
	}

	// Short-circuit if we are already on the target branch to avoid timestamp/status bugs on startup
	if headReference.Name() == threadBranch {
		return nil
	}

	worktree, err := repository.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree: %w", err)
	}

	err = worktree.Checkout(&git.CheckoutOptions{
		Hash:   headReference.Hash(),
		Branch: threadBranch,
		Create: true,
		Keep:   true,
	})
	if err != nil {
		if err := worktree.Checkout(&git.CheckoutOptions{
			Branch: threadBranch,
			Keep:   true,
		}); err != nil {
			return fmt.Errorf("checking out thread branch: %w", err)
		}
	}

	return nil
}

func (c *GoGitChat) Snapshot(ctx context.Context, chatID string, message string) (string, error) {
	repository, err := git.PlainOpen(c.workspaceRoot)
	if err != nil {
		return "", fmt.Errorf("opening repository: %w", err)
	}

	worktree, err := repository.Worktree()
	if err != nil {
		return "", fmt.Errorf("getting worktree: %w", err)
	}

	if err := worktree.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		return "", fmt.Errorf("staging all files: %w", err)
	}

	commitHash, err := worktree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Workspace Engine",
			Email: "engine@local.workspace",
			When:  time.Now().UTC(),
		},
		AllowEmptyCommits: true,
	})
	if err != nil {
		return "", fmt.Errorf("committing checkpoint: %w", err)
	}

	return commitHash.String(), nil
}

func (c *GoGitChat) SpawnCandidateSandbox(ctx context.Context, chatID string, candidateID string) (workspace.CandidateSandbox, error) {
	sandboxDirectory, err := os.MkdirTemp("", fmt.Sprintf("sandbox-%s-*", candidateID))
	if err != nil {
		return nil, fmt.Errorf("creating temp sandbox directory: %w", err)
	}

	repository, err := git.PlainCloneContext(ctx, sandboxDirectory, false, &git.CloneOptions{
		URL: c.workspaceRoot,
	})
	if err != nil {
		return nil, fmt.Errorf("cloning repository to sandbox: %w", err)
	}

	worktree, err := repository.Worktree()
	if err != nil {
		return nil, fmt.Errorf("getting sandbox worktree: %w", err)
	}

	threadBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("chat/%s", chatID))
	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: threadBranch,
	})
	if err != nil {
		return nil, fmt.Errorf("checking out thread branch in sandbox: %w", err)
	}

	headReference, err := repository.Head()
	if err != nil {
		return nil, fmt.Errorf("resolving sandbox HEAD: %w", err)
	}

	candidateBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("candidate/%s", candidateID))
	if err := worktree.Checkout(&git.CheckoutOptions{
		Hash:   headReference.Hash(),
		Branch: candidateBranch,
		Create: true,
	}); err != nil {
		return nil, fmt.Errorf("creating sandbox candidate branch: %w", err)
	}

	return &goGitSandbox{
		workspaceRoot:    c.workspaceRoot,
		sandboxDirectory: sandboxDirectory,
		chatDirectory:    filepath.Join(sandboxDirectory, "chats", chatID),
		candidateID:      candidateID,
		sharedCodebase:   c.sharedCodebase,
	}, nil
}

func (c *GoGitChat) PreviewCandidate(ctx context.Context, chatID string, candidateID string) error {
	repository, err := git.PlainOpen(c.workspaceRoot)
	if err != nil {
		return fmt.Errorf("opening repository: %w", err)
	}
	worktree, err := repository.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree: %w", err)
	}

	candidateBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("candidate/%s", candidateID))

	// Safely float the ledger in memory and explicitly force the worktree to match the candidate
	return c.withFloatingLedger(worktree, func() error {
		return worktree.Checkout(&git.CheckoutOptions{
			Branch: candidateBranch,
			Force:  true,
		})
	})
}

func (c *GoGitChat) ReadCandidateDiff(ctx context.Context, chatID string, candidateID string) (string, error) {
	repository, err := git.PlainOpen(c.workspaceRoot)
	if err != nil {
		return "", fmt.Errorf("opening repository: %w", err)
	}

	threadBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("chat/%s", chatID))
	candidateBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("candidate/%s", candidateID))

	threadReference, err := repository.Reference(threadBranch, true)
	if err != nil {
		return "", fmt.Errorf("resolving thread branch: %w", err)
	}

	candidateReference, err := repository.Reference(candidateBranch, true)
	if err != nil {
		return "", fmt.Errorf("resolving candidate branch: %w", err)
	}

	threadCommit, err := repository.CommitObject(threadReference.Hash())
	if err != nil {
		return "", fmt.Errorf("getting thread commit: %w", err)
	}

	candidateCommit, err := repository.CommitObject(candidateReference.Hash())
	if err != nil {
		return "", fmt.Errorf("getting candidate commit: %w", err)
	}

	threadTree, err := threadCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("getting thread tree: %w", err)
	}

	candidateTree, err := candidateCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("getting candidate tree: %w", err)
	}

	patch, err := threadTree.Patch(candidateTree)
	if err != nil {
		return "", fmt.Errorf("generating patch: %w", err)
	}

	return patch.String(), nil
}

func (c *GoGitChat) createAnnotatedTag(repository *git.Repository, targetHash plumbing.Hash, candidateID string, reason string) error {
	tagName := fmt.Sprintf("proposal-%s", candidateID)

	tagObj := &object.Tag{
		Name:       tagName,
		Message:    reason,
		TargetType: plumbing.CommitObject,
		Target:     targetHash,
		Tagger: object.Signature{
			Name:  "Main Session",
			Email: "session@local.workspace",
			When:  time.Now().UTC(),
		},
	}

	obj := repository.Storer.NewEncodedObject()
	if err := tagObj.Encode(obj); err != nil {
		return err
	}

	tagHash, err := repository.Storer.SetEncodedObject(obj)
	if err != nil {
		return err
	}

	referenceName := plumbing.ReferenceName(fmt.Sprintf("refs/tags/%s", tagName))
	reference := plumbing.NewHashReference(referenceName, tagHash)

	return repository.Storer.SetReference(reference)
}

func (c *GoGitChat) Accept(ctx context.Context, chatID string, candidateID string, reason string) error {
	repository, err := git.PlainOpen(c.workspaceRoot)
	if err != nil {
		return fmt.Errorf("opening repository: %w", err)
	}
	worktree, err := repository.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree: %w", err)
	}

	threadBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("chat/%s", chatID))
	candidateBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("candidate/%s", candidateID))

	candidateReference, err := repository.Reference(candidateBranch, true)
	if err != nil {
		return fmt.Errorf("resolving candidate branch: %w", err)
	}

	// 1. Fast-forward the thread branch reference
	fastForwardReference := plumbing.NewHashReference(threadBranch, candidateReference.Hash())
	if err := repository.Storer.SetReference(fastForwardReference); err != nil {
		return fmt.Errorf("fast-forwarding branch reference: %w", err)
	}

	// 2. Tag the acceptance permanently
	if err := c.createAnnotatedTag(repository, candidateReference.Hash(), candidateID, fmt.Sprintf("ACCEPTED: %s", reason)); err != nil {
		return fmt.Errorf("creating accepted tag: %w", err)
	}

	// 3. Update HEAD to symbolically point back to the thread branch
	headReference := plumbing.NewSymbolicReference(plumbing.HEAD, threadBranch)
	if err := repository.Storer.SetReference(headReference); err != nil {
		return fmt.Errorf("updating HEAD reference: %w", err)
	}

	// 4. Force checkout the thread branch to guarantee file extraction, wrapping it to float the ledger
	if err := c.withFloatingLedger(worktree, func() error {
		return worktree.Checkout(&git.CheckoutOptions{
			Branch: threadBranch,
			Force:  true,
		})
	}); err != nil {
		return fmt.Errorf("syncing worktree after fast-forward: %w", err)
	}

	_ = repository.Storer.RemoveReference(candidateBranch)
	return nil
}

func (c *GoGitChat) Reject(ctx context.Context, chatID string, candidateID string, reason string) error {
	repository, err := git.PlainOpen(c.workspaceRoot)
	if err != nil {
		return fmt.Errorf("opening repository: %w", err)
	}
	worktree, err := repository.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree: %w", err)
	}

	threadBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("chat/%s", chatID))

	// Return to the thread branch safely
	if err := c.withFloatingLedger(worktree, func() error {
		return worktree.Checkout(&git.CheckoutOptions{
			Branch: threadBranch,
			Force:  true,
		})
	}); err != nil {
		return fmt.Errorf("returning to thread branch: %w", err)
	}

	candidateBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("candidate/%s", candidateID))
	candidateReference, err := repository.Reference(candidateBranch, true)
	if err == nil {
		if err := c.createAnnotatedTag(repository, candidateReference.Hash(), candidateID, fmt.Sprintf("REJECTED: %s", reason)); err != nil {
			return fmt.Errorf("creating rejected tag: %w", err)
		}
		if err := repository.Storer.RemoveReference(candidateBranch); err != nil {
			return fmt.Errorf("removing candidate branch reference: %w", err)
		}
	}

	return nil
}

// --- Sandbox Virtual Environment ---

// goGitSandbox implements the workspace.CandidateSandbox interface.
type goGitSandbox struct {
	workspaceRoot    string
	sandboxDirectory string
	chatDirectory    string
	candidateID      string
	sharedCodebase   bool
}

// normalizePath safely forces incoming paths into the configured execution root,
// stripping out redundant prefixes if the test or prompt already supplied them.
func (s *goGitSandbox) normalizePath(targetPath string) string {
	slashPath := filepath.ToSlash(filepath.Clean(targetPath))
	chatID := filepath.Base(s.chatDirectory)
	slashPrefix := fmt.Sprintf("chats/%s", chatID)

	if strings.HasPrefix(slashPath, slashPrefix) {
		slashPath = strings.TrimPrefix(slashPath, slashPrefix)
		slashPath = strings.TrimPrefix(slashPath, "/")
	}

	if s.sharedCodebase {
		return filepath.Join(s.sandboxDirectory, filepath.FromSlash(slashPath))
	}
	return filepath.Join(s.chatDirectory, filepath.FromSlash(slashPath))
}

func (s *goGitSandbox) WriteFile(ctx context.Context, path string, data []byte) error {
	fullPath := s.normalizePath(path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, data, 0644)
}

func (s *goGitSandbox) ReadFile(ctx context.Context, path string) ([]byte, error) {
	return os.ReadFile(s.normalizePath(path))
}

func (s *goGitSandbox) ExecuteCommand(ctx context.Context, command string, args ...string) (string, error) {
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

func (s *goGitSandbox) ApplyDraft(ctx context.Context, message string) error {
	repository, err := git.PlainOpen(s.sandboxDirectory)
	if err != nil {
		return fmt.Errorf("opening sandbox repository: %w", err)
	}

	worktree, err := repository.Worktree()
	if err != nil {
		return fmt.Errorf("getting sandbox worktree: %w", err)
	}

	// Force go-git to index newly created deeply nested directories
	_, _ = worktree.Add(".")
	if err := worktree.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		return fmt.Errorf("staging files in sandbox: %w", err)
	}

	_, err = worktree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Agent Workspace",
			Email: "agent@local.workspace",
			When:  time.Now().UTC(),
		},
	})
	if err != nil {
		return fmt.Errorf("committing sandbox changes: %w", err)
	}

	return nil
}

func (s *goGitSandbox) DeliverForReview(ctx context.Context) error {
	repository, err := git.PlainOpen(s.sandboxDirectory)
	if err != nil {
		return fmt.Errorf("opening sandbox repository: %w", err)
	}

	referenceName := fmt.Sprintf("refs/heads/candidate/%s", s.candidateID)

	err = repository.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs: []config.RefSpec{
			config.RefSpec(fmt.Sprintf("%s:%s", referenceName, referenceName)),
		},
	})
	if err != nil {
		return fmt.Errorf("pushing sandbox to main directory: %w", err)
	}

	return nil
}

func (s *goGitSandbox) TearDown(ctx context.Context) error {
	return os.RemoveAll(s.sandboxDirectory)
}
