package gitfs

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GoGitEngine implements the workspace.StateEngine interface using the go-git library.
type GoGitEngine struct{}

func NewGoGitEngine() *GoGitEngine {
	return &GoGitEngine{}
}

func (e *GoGitEngine) InitThread(ctx context.Context, mainDir string, threadID string) error {
	repository, err := git.PlainInit(mainDir, false)
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
		repository, err = git.PlainOpen(mainDir)
		if err != nil {
			return fmt.Errorf("opening repository: %w", err)
		}
	}

	threadBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("chat/%s", threadID))
	headRef, err := repository.Head()
	if err != nil {
		return fmt.Errorf("resolving HEAD: %w", err)
	}

	worktree, err := repository.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree: %w", err)
	}

	err = worktree.Checkout(&git.CheckoutOptions{
		Hash:   headRef.Hash(),
		Branch: threadBranch,
		Create: true,
	})
	if err != nil {
		if err := worktree.Checkout(&git.CheckoutOptions{Branch: threadBranch}); err != nil {
			return fmt.Errorf("checking out thread branch: %w", err)
		}
	}

	return nil
}

func (e *GoGitEngine) Snapshot(ctx context.Context, mainDir string, threadID string, message string) (string, error) {
	repository, err := git.PlainOpen(mainDir)
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

// --- Agentic Sandbox Management ---

func (e *GoGitEngine) SpawnSandbox(ctx context.Context, mainDir string, threadID string, candidateID string) (string, error) {
	sandboxDir, err := os.MkdirTemp("", fmt.Sprintf("sandbox-%s-*", candidateID))
	if err != nil {
		return "", fmt.Errorf("creating temp sandbox dir: %w", err)
	}

	// 1. Clone the repository locally
	repository, err := git.PlainCloneContext(ctx, sandboxDir, false, &git.CloneOptions{
		URL: mainDir,
	})
	if err != nil {
		return "", fmt.Errorf("cloning repository to sandbox: %w", err)
	}

	worktree, err := repository.Worktree()
	if err != nil {
		return "", fmt.Errorf("getting sandbox worktree: %w", err)
	}

	// 2. Resolve the thread branch
	threadBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("chat/%s", threadID))
	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: threadBranch,
	})
	if err != nil {
		return "", fmt.Errorf("checking out thread branch in sandbox: %w", err)
	}

	headRef, err := repository.Head()
	if err != nil {
		return "", fmt.Errorf("resolving sandbox HEAD: %w", err)
	}

	// 3. Create and switch to candidate branch
	candidateBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("candidate/%s", candidateID))
	if err := worktree.Checkout(&git.CheckoutOptions{
		Hash:   headRef.Hash(),
		Branch: candidateBranch,
		Create: true,
	}); err != nil {
		return "", fmt.Errorf("creating sandbox candidate branch: %w", err)
	}

	return sandboxDir, nil
}

func (e *GoGitEngine) CommitSandbox(ctx context.Context, sandboxDir string, message string) (string, error) {
	repository, err := git.PlainOpen(sandboxDir)
	if err != nil {
		return "", fmt.Errorf("opening sandbox repository: %w", err)
	}

	worktree, err := repository.Worktree()
	if err != nil {
		return "", fmt.Errorf("getting sandbox worktree: %w", err)
	}

	if err := worktree.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		return "", fmt.Errorf("staging files in sandbox: %w", err)
	}

	commitHash, err := worktree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Agent Workspace",
			Email: "agent@local.workspace",
			When:  time.Now().UTC(),
		},
	})
	if err != nil {
		return "", fmt.Errorf("committing sandbox changes: %w", err)
	}

	return commitHash.String(), nil
}

func (e *GoGitEngine) SubmitSandbox(ctx context.Context, sandboxDir string, mainDir string, candidateID string) error {
	repository, err := git.PlainOpen(sandboxDir)
	if err != nil {
		return fmt.Errorf("opening sandbox repository: %w", err)
	}

	refName := fmt.Sprintf("refs/heads/candidate/%s", candidateID)

	// Push the local candidate branch back to the main thread repository
	err = repository.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs: []config.RefSpec{
			config.RefSpec(fmt.Sprintf("%s:%s", refName, refName)),
		},
	})
	if err != nil {
		return fmt.Errorf("pushing sandbox to main directory: %w", err)
	}

	return nil
}

func (e *GoGitEngine) CloseSandbox(ctx context.Context, sandboxDir string) error {
	return os.RemoveAll(sandboxDir)
}

// --- Main Session / User Review ---

func (e *GoGitEngine) ReadCandidateDiff(ctx context.Context, mainDir string, threadID string, candidateID string) (string, error) {
	repository, err := git.PlainOpen(mainDir)
	if err != nil {
		return "", fmt.Errorf("opening repository: %w", err)
	}

	threadBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("chat/%s", threadID))
	candidateBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("candidate/%s", candidateID))

	threadRef, err := repository.Reference(threadBranch, true)
	if err != nil {
		return "", fmt.Errorf("resolving thread branch: %w", err)
	}

	candidateRef, err := repository.Reference(candidateBranch, true)
	if err != nil {
		return "", fmt.Errorf("resolving candidate branch: %w", err)
	}

	threadCommit, err := repository.CommitObject(threadRef.Hash())
	if err != nil {
		return "", fmt.Errorf("getting thread commit: %w", err)
	}

	candidateCommit, err := repository.CommitObject(candidateRef.Hash())
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

func (e *GoGitEngine) PreviewCandidate(ctx context.Context, mainDir string, threadID string, candidateID string) error {
	repository, err := git.PlainOpen(mainDir)
	if err != nil {
		return fmt.Errorf("opening repository: %w", err)
	}
	worktree, err := repository.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree: %w", err)
	}

	candidateBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("candidate/%s", candidateID))

	// Safely checks out the candidate branch, floating the uncommitted ledger natively
	if err := worktree.Checkout(&git.CheckoutOptions{
		Branch: candidateBranch,
	}); err != nil {
		return fmt.Errorf("checking out candidate for preview: %w", err)
	}

	return nil
}

// createAnnotatedTag is a private helper to permanently record accept/reject decisions.
func (e *GoGitEngine) createAnnotatedTag(repository *git.Repository, targetHash plumbing.Hash, candidateID string, reason string) error {
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

	refName := plumbing.ReferenceName(fmt.Sprintf("refs/tags/%s", tagName))
	ref := plumbing.NewHashReference(refName, tagHash)

	return repository.Storer.SetReference(ref)
}

func (e *GoGitEngine) Accept(ctx context.Context, mainDir string, threadID string, candidateID string, reason string) error {
	repository, err := git.PlainOpen(mainDir)
	if err != nil {
		return fmt.Errorf("opening repository: %w", err)
	}
	worktree, err := repository.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree: %w", err)
	}

	threadBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("chat/%s", threadID))
	candidateBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("candidate/%s", candidateID))

	candidateRef, err := repository.Reference(candidateBranch, true)
	if err != nil {
		return fmt.Errorf("resolving candidate branch: %w", err)
	}

	if err := worktree.Checkout(&git.CheckoutOptions{Branch: threadBranch}); err != nil {
		return fmt.Errorf("checkout thread branch before merge: %w", err)
	}

	fastForwardReference := plumbing.NewHashReference(threadBranch, candidateRef.Hash())
	if err := repository.Storer.SetReference(fastForwardReference); err != nil {
		return fmt.Errorf("fast-forwarding branch reference: %w", err)
	}

	if err := e.createAnnotatedTag(repository, candidateRef.Hash(), candidateID, fmt.Sprintf("ACCEPTED: %s", reason)); err != nil {
		return fmt.Errorf("creating accepted tag: %w", err)
	}

	if err := worktree.Checkout(&git.CheckoutOptions{Branch: threadBranch}); err != nil {
		return fmt.Errorf("syncing worktree after fast-forward: %w", err)
	}

	_ = repository.Storer.RemoveReference(candidateBranch)
	return nil
}

func (e *GoGitEngine) Reject(ctx context.Context, mainDir string, threadID string, candidateID string, reason string) error {
	repository, err := git.PlainOpen(mainDir)
	if err != nil {
		return fmt.Errorf("opening repository: %w", err)
	}
	worktree, err := repository.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree: %w", err)
	}

	threadBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("chat/%s", threadID))

	if err := worktree.Checkout(&git.CheckoutOptions{Branch: threadBranch}); err != nil {
		return fmt.Errorf("returning to thread branch: %w", err)
	}

	candidateBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("candidate/%s", candidateID))
	candidateRef, err := repository.Reference(candidateBranch, true)
	if err == nil {
		if err := e.createAnnotatedTag(repository, candidateRef.Hash(), candidateID, fmt.Sprintf("REJECTED: %s", reason)); err != nil {
			return fmt.Errorf("creating rejected tag: %w", err)
		}
		if err := repository.Storer.RemoveReference(candidateBranch); err != nil {
			return fmt.Errorf("removing candidate branch reference: %w", err)
		}
	}

	return nil
}
