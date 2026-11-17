package gitfs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// GoGitEngine implements the workspace.ChatVersioner interface using the go-git library.
type GoGitEngine struct {
	mu             sync.Mutex
	workspaceRoot  string
	sharedCodebase bool
}

func NewGoGitEngine(workspaceRoot string, sharedCodebase bool) *GoGitEngine {
	return &GoGitEngine{
		workspaceRoot:  workspaceRoot,
		sharedCodebase: sharedCodebase,
	}
}

// withFloatingLedger stashes uncommitted/untracked files (such as the conversation ledger)
// before forcing a clean branch checkout, then restores them to disk.
func (c *GoGitEngine) withFloatingLedger(worktree *git.Worktree, action func() error) error {
	status, err := worktree.Status()
	if err != nil {
		return fmt.Errorf("checking worktree status before checkout: %w", err)
	}

	stash := make(map[string][]byte)
	for path, fileStatus := range status {
		if fileStatus.Worktree == git.Modified || fileStatus.Worktree == git.Untracked {
			fullPath := filepath.Join(c.workspaceRoot, filepath.FromSlash(path))
			info, statErr := os.Stat(fullPath)
			if statErr != nil {
				if errors.Is(statErr, os.ErrNotExist) {
					continue
				}
				return fmt.Errorf("statting floating ledger file %s: %w", path, statErr)
			}
			if info.IsDir() {
				continue
			}
			data, readErr := os.ReadFile(fullPath)
			if readErr != nil {
				return fmt.Errorf("reading floating ledger file %s: %w", path, readErr)
			}
			stash[path] = data
		}
	}

	if err := action(); err != nil {
		return err
	}

	for path, data := range stash {
		fullPath := filepath.Join(c.workspaceRoot, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return fmt.Errorf("creating directory for floating ledger %s: %w", path, err)
		}
		if err := os.WriteFile(fullPath, data, 0644); err != nil {
			return fmt.Errorf("restoring floating ledger %s: %w", path, err)
		}
	}

	return nil
}

func (c *GoGitEngine) InitChat(ctx context.Context, chatID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	repository, err := git.PlainInit(c.workspaceRoot, false)
	if err != nil && !errors.Is(err, git.ErrRepositoryAlreadyExists) {
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
		if checkoutErr := worktree.Checkout(&git.CheckoutOptions{
			Branch: threadBranch,
			Keep:   true,
		}); checkoutErr != nil {
			return fmt.Errorf("checking out thread branch: %w", checkoutErr)
		}
	}

	return nil
}

func (c *GoGitEngine) Snapshot(ctx context.Context, chatID string, message string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()

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

func (c *GoGitEngine) SpawnCandidateSandbox(ctx context.Context, chatID string, candidateID string) (workspace.CandidateSandbox, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	sandboxDirectory, err := os.MkdirTemp("", fmt.Sprintf("sandbox-%s-*", candidateID))
	if err != nil {
		return nil, fmt.Errorf("creating temp sandbox directory: %w", err)
	}

	cleanupOnErr := func() {
		_ = os.RemoveAll(sandboxDirectory)
	}

	repository, err := git.PlainCloneContext(ctx, sandboxDirectory, false, &git.CloneOptions{
		URL: c.workspaceRoot,
	})
	if err != nil {
		cleanupOnErr()
		return nil, fmt.Errorf("cloning repository to sandbox: %w", err)
	}

	worktree, err := repository.Worktree()
	if err != nil {
		cleanupOnErr()
		return nil, fmt.Errorf("getting sandbox worktree: %w", err)
	}

	threadBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("chat/%s", chatID))
	if err := worktree.Checkout(&git.CheckoutOptions{
		Branch: threadBranch,
	}); err != nil {
		cleanupOnErr()
		return nil, fmt.Errorf("checking out thread branch in sandbox: %w", err)
	}

	headReference, err := repository.Head()
	if err != nil {
		cleanupOnErr()
		return nil, fmt.Errorf("resolving sandbox HEAD: %w", err)
	}

	candidateBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("candidate/%s", candidateID))
	if err := worktree.Checkout(&git.CheckoutOptions{
		Hash:   headReference.Hash(),
		Branch: candidateBranch,
		Create: true,
	}); err != nil {
		cleanupOnErr()
		return nil, fmt.Errorf("creating sandbox candidate branch: %w", err)
	}

	return &goGitSandbox{
		repoMu:           &c.mu,
		workspaceRoot:    c.workspaceRoot,
		sandboxDirectory: sandboxDirectory,
		chatDirectory:    filepath.Join(sandboxDirectory, "chats", chatID),
		candidateID:      candidateID,
		sharedCodebase:   c.sharedCodebase,
	}, nil
}

func (c *GoGitEngine) PreviewCandidate(ctx context.Context, chatID string, candidateID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	repository, err := git.PlainOpen(c.workspaceRoot)
	if err != nil {
		return fmt.Errorf("opening repository: %w", err)
	}
	worktree, err := repository.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree: %w", err)
	}

	candidateBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("candidate/%s", candidateID))

	return c.withFloatingLedger(worktree, func() error {
		return worktree.Checkout(&git.CheckoutOptions{
			Branch: candidateBranch,
			Force:  true,
		})
	})
}

func (c *GoGitEngine) ReadCandidateDiff(ctx context.Context, chatID string, candidateID string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	repository, err := git.PlainOpen(c.workspaceRoot)
	if err != nil {
		return "", fmt.Errorf("opening repository: %w", err)
	}

	threadBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("chat/%s", chatID))
	candidateBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("candidate/%s", candidateID))

	candidateReference, err := repository.Reference(candidateBranch, true)
	if err == nil {
		threadReference, threadErr := repository.Reference(threadBranch, true)
		if threadErr != nil {
			return "", fmt.Errorf("resolving thread branch: %w", threadErr)
		}

		threadCommit, commitErr := repository.CommitObject(threadReference.Hash())
		if commitErr != nil {
			return "", fmt.Errorf("getting thread commit: %w", commitErr)
		}

		candidateCommit, candCommitErr := repository.CommitObject(candidateReference.Hash())
		if candCommitErr != nil {
			return "", fmt.Errorf("getting candidate commit: %w", candCommitErr)
		}

		threadTree, treeErr := threadCommit.Tree()
		if treeErr != nil {
			return "", fmt.Errorf("getting thread tree: %w", treeErr)
		}

		candidateTree, candTreeErr := candidateCommit.Tree()
		if candTreeErr != nil {
			return "", fmt.Errorf("getting candidate tree: %w", candTreeErr)
		}

		patch, patchErr := threadTree.Patch(candidateTree)
		if patchErr != nil {
			return "", fmt.Errorf("generating patch: %w", patchErr)
		}

		return patch.String(), nil
	} else if !errors.Is(err, plumbing.ErrReferenceNotFound) {
		return "", fmt.Errorf("looking up candidate branch %s: %w", candidateBranch, err)
	}

	tagRefName := plumbing.ReferenceName(fmt.Sprintf("refs/tags/proposal-%s", candidateID))
	tagReference, err := repository.Reference(tagRefName, true)
	if err != nil {
		return "", fmt.Errorf("candidate %s not found as active branch or proposal tag: %w", candidateID, err)
	}

	commitHash := tagReference.Hash()
	if tagObj, tagErr := repository.TagObject(commitHash); tagErr == nil {
		commitHash = tagObj.Target
	}

	candidateCommit, err := repository.CommitObject(commitHash)
	if err != nil {
		return "", fmt.Errorf("getting tagged candidate commit: %w", err)
	}

	candidateTree, err := candidateCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("getting candidate tree: %w", err)
	}

	parentTree := &object.Tree{}
	if candidateCommit.NumParents() > 0 {
		parentCommit, parentErr := candidateCommit.Parent(0)
		if parentErr != nil {
			return "", fmt.Errorf("getting parent commit for tagged candidate: %w", parentErr)
		}
		parentTree, err = parentCommit.Tree()
		if err != nil {
			return "", fmt.Errorf("getting parent tree: %w", err)
		}
	}

	patch, err := parentTree.Patch(candidateTree)
	if err != nil {
		return "", fmt.Errorf("generating patch from proposal tag: %w", err)
	}

	return patch.String(), nil
}

func (c *GoGitEngine) createAnnotatedTag(repository *git.Repository, targetHash plumbing.Hash, candidateID string, reason string) error {
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

func (c *GoGitEngine) Accept(ctx context.Context, chatID string, candidateID string, reason string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()

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

	fastForwardReference := plumbing.NewHashReference(threadBranch, candidateReference.Hash())
	if err := repository.Storer.SetReference(fastForwardReference); err != nil {
		return fmt.Errorf("fast-forwarding branch reference: %w", err)
	}

	if err := c.createAnnotatedTag(repository, candidateReference.Hash(), candidateID, fmt.Sprintf("ACCEPTED: %s", reason)); err != nil {
		return fmt.Errorf("creating accepted tag: %w", err)
	}

	headReference := plumbing.NewSymbolicReference(plumbing.HEAD, threadBranch)
	if err := repository.Storer.SetReference(headReference); err != nil {
		return fmt.Errorf("updating HEAD reference: %w", err)
	}

	if err := c.withFloatingLedger(worktree, func() error {
		return worktree.Checkout(&git.CheckoutOptions{
			Branch: threadBranch,
			Force:  true,
		})
	}); err != nil {
		return fmt.Errorf("syncing worktree after fast-forward: %w", err)
	}

	if err := repository.Storer.RemoveReference(candidateBranch); err != nil {
		return fmt.Errorf("removing accepted candidate branch reference: %w", err)
	}
	return nil
}

func (c *GoGitEngine) Reject(ctx context.Context, chatID string, candidateID string, reason string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	repository, err := git.PlainOpen(c.workspaceRoot)
	if err != nil {
		return fmt.Errorf("opening repository: %w", err)
	}
	worktree, err := repository.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree: %w", err)
	}

	threadBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("chat/%s", chatID))

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
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return nil
		}
		return fmt.Errorf("resolving candidate branch %s for rejection: %w", candidateBranch, err)
	}

	if err := c.createAnnotatedTag(repository, candidateReference.Hash(), candidateID, fmt.Sprintf("REJECTED: %s", reason)); err != nil {
		return fmt.Errorf("creating rejected tag: %w", err)
	}
	if err := repository.Storer.RemoveReference(candidateBranch); err != nil {
		return fmt.Errorf("removing candidate branch reference: %w", err)
	}

	return nil
}

type goGitSandbox struct {
	repoMu           *sync.Mutex
	workspaceRoot    string
	sandboxDirectory string
	chatDirectory    string
	candidateID      string
	sharedCodebase   bool
}

func (s *goGitSandbox) resolveSafePath(targetPath string) (string, error) {
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

func (s *goGitSandbox) ResolvePath(path string) string {
	fullPath, err := s.resolveSafePath(path)
	if err != nil {
		if s.sharedCodebase {
			return s.sandboxDirectory
		}
		return s.chatDirectory
	}
	return fullPath
}

func (s *goGitSandbox) WriteFile(ctx context.Context, path string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	fullPath, err := s.resolveSafePath(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, data, 0644)
}

func (s *goGitSandbox) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fullPath, err := s.resolveSafePath(path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(fullPath)
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
	if err := ctx.Err(); err != nil {
		return err
	}
	repository, err := git.PlainOpen(s.sandboxDirectory)
	if err != nil {
		return fmt.Errorf("opening sandbox repository: %w", err)
	}

	worktree, err := repository.Worktree()
	if err != nil {
		return fmt.Errorf("getting sandbox worktree: %w", err)
	}

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
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.repoMu != nil {
		s.repoMu.Lock()
		defer s.repoMu.Unlock()
	}

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
