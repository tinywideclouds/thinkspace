package workspace

import "context"

// StateEngine manages the physical storage of the chat's thoughts and code.
// The application interacts with this interface strictly in domain terms.
type StateEngine interface {
	// InitThread establishes the base state for a new conversational thread.
	InitThread(ctx context.Context, dir string, threadID string) error

	// Snapshot permanently saves the accepted state of the thread (both code and ledger).
	Snapshot(ctx context.Context, dir string, threadID string, message string) (string, error)

	// Propose creates an isolated sandbox, writes the generated code, saves it permanently
	// for future reference, and then seamlessly returns the worktree to the thread's state.
	// It guarantees that active chat ledger writes are not disrupted.
	Propose(ctx context.Context, dir string, threadID string, proposalID string, files map[string][]byte, toolName string) (string, error)

	// Accept integrates a proposed set of changes into the thread's accepted reality.
	Accept(ctx context.Context, dir string, threadID string, proposalID string) error

	// Reject cleans up the active sandbox for a proposal, though the data remains
	// accessible via the storage layer's permanent tagging mechanism.
	Reject(ctx context.Context, dir string, threadID string, proposalID string) error
}
