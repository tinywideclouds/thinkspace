package gitfs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// NativeEngine implements the workspace.StateEngine interface using pure Go.
type NativeEngine struct{}

func NewNativeEngine() *NativeEngine {
	return &NativeEngine{}
}

// InitThread establishes the base Git state for a new conversational thread.
func (e *NativeEngine) InitThread(ctx context.Context, dir string, threadID string) error {
	repository, err := git.PlainInit(dir, false)
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
		repository, err = git.PlainOpen(dir)
		if err != nil {
			return fmt.Errorf("opening repository: %w", err)
		}
	}

	threadBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("chat/%s", threadID))
	headRef, err := repository.Head()
	if err != nil {
		return fmt.Errorf("resolving HEAD: %w", err)
	}

	// Create and switch to the thread branch
	worktree, err := repository.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree: %w", err)
	}

	err = worktree.Checkout(&git.CheckoutOptions{
		Hash:   headRef.Hash(),
		Branch: threadBranch,
		Create: true,
	})
	// Ignore err if branch already exists, we will just check it out
	if err != nil {
		if err := worktree.Checkout(&git.CheckoutOptions{Branch: threadBranch}); err != nil {
			return fmt.Errorf("checking out thread branch: %w", err)
		}
	}

	return nil
}

// Snapshot permanently saves the accepted state of the thread (both code and ledger).
func (e *NativeEngine) Snapshot(ctx context.Context, dir string, threadID string, message string) (string, error) {
	repository, err := git.PlainOpen(dir)
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

// Propose creates an isolated sandbox, writes the generated code, saves it permanently,
// and deliberately leaves the working tree on this candidate branch for user review.
func (e *NativeEngine) Propose(ctx context.Context, dir string, threadID string, proposalID string, files map[string][]byte, toolName string) (string, error) {
	repository, err := git.PlainOpen(dir)
	if err != nil {
		return "", fmt.Errorf("opening repository: %w", err)
	}
	worktree, err := repository.Worktree()
	if err != nil {
		return "", fmt.Errorf("getting worktree: %w", err)
	}

	threadBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("chat/%s", threadID))
	candidateBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("candidate/%s", proposalID))

	headRef, err := repository.Reference(threadBranch, true)
	if err != nil {
		return "", fmt.Errorf("resolving thread branch: %w", err)
	}

	// 1. The `git switch -c` equivalent. Safely floats the uncommitted ledger.
	if err := worktree.Checkout(&git.CheckoutOptions{
		Hash:   headRef.Hash(),
		Branch: candidateBranch,
		Create: true,
	}); err != nil {
		return "", fmt.Errorf("creating candidate branch: %w", err)
	}

	// 2. Write and track the files
	for relativePath, content := range files {
		cleanPath := filepath.Clean(relativePath)
		absolutePath := filepath.Join(dir, "chats", threadID, "docs", cleanPath)

		if err := os.MkdirAll(filepath.Dir(absolutePath), 0755); err != nil {
			return "", fmt.Errorf("creating directory for %s: %w", cleanPath, err)
		}
		if err := os.WriteFile(absolutePath, content, 0644); err != nil {
			return "", fmt.Errorf("writing file %s: %w", cleanPath, err)
		}

		trackPath := filepath.Join("chats", threadID, "docs", cleanPath)
		if _, err := worktree.Add(trackPath); err != nil {
			return "", fmt.Errorf("staging proposed file %s: %w", cleanPath, err)
		}
	}

	// 3. Commit the changes
	commitMessage := fmt.Sprintf("auto(candidate): generated by %s", toolName)
	commitHash, err := worktree.Commit(commitMessage, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "LLM Tool",
			Email: "tool@local.workspace",
			When:  time.Now().UTC(),
		},
	})
	if err != nil {
		return "", fmt.Errorf("committing proposal: %w", err)
	}

	// 4. Tag it permanently
	tagReferenceName := plumbing.ReferenceName(fmt.Sprintf("refs/tags/proposal-%s", proposalID))
	tagReference := plumbing.NewHashReference(tagReferenceName, commitHash)
	if err := repository.Storer.SetReference(tagReference); err != nil {
		return "", fmt.Errorf("setting tag reference: %w", err)
	}

	return commitHash.String(), nil
}

// Accept integrates a proposed set of changes into the thread's accepted reality.
func (e *NativeEngine) Accept(ctx context.Context, dir string, threadID string, proposalID string) error {
	repository, err := git.PlainOpen(dir)
	if err != nil {
		return fmt.Errorf("opening repository: %w", err)
	}
	worktree, err := repository.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree: %w", err)
	}

	threadBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("chat/%s", threadID))
	candidateBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("candidate/%s", proposalID))

	candidateRef, err := repository.Reference(candidateBranch, true)
	if err != nil {
		return fmt.Errorf("resolving candidate branch: %w", err)
	}

	// 1. Switch back to thread branch. Floats the ledger natively. Temporarily removes docs/.
	if err := worktree.Checkout(&git.CheckoutOptions{Branch: threadBranch}); err != nil {
		return fmt.Errorf("checkout thread branch before merge: %w", err)
	}

	// 2. Fast-Forward the thread branch pointer.
	fastForwardReference := plumbing.NewHashReference(threadBranch, candidateRef.Hash())
	if err := repository.Storer.SetReference(fastForwardReference); err != nil {
		return fmt.Errorf("fast-forwarding branch reference: %w", err)
	}

	// 3. Sync the worktree to the new pointer. Restores docs/. Floats the ledger seamlessly.
	if err := worktree.Checkout(&git.CheckoutOptions{Branch: threadBranch}); err != nil {
		return fmt.Errorf("syncing worktree after fast-forward: %w", err)
	}

	// 4. Clean up
	_ = repository.Storer.RemoveReference(candidateBranch)

	return nil
}

// Reject discards the proposal and cleans up the active sandbox.
func (e *NativeEngine) Reject(ctx context.Context, dir string, threadID string, proposalID string) error {
	repository, err := git.PlainOpen(dir)
	if err != nil {
		return fmt.Errorf("opening repository: %w", err)
	}
	worktree, err := repository.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree: %w", err)
	}

	threadBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("chat/%s", threadID))

	// 1. Switch back to thread branch. Natively floats ledger AND cleanly deletes rejected docs/.
	if err := worktree.Checkout(&git.CheckoutOptions{Branch: threadBranch}); err != nil {
		return fmt.Errorf("returning to thread branch: %w", err)
	}

	candidateBranch := plumbing.NewBranchReferenceName(fmt.Sprintf("candidate/%s", proposalID))

	// 2. Clean up
	if err := repository.Storer.RemoveReference(candidateBranch); err != nil {
		return fmt.Errorf("removing candidate branch reference: %w", err)
	}

	return nil
}
