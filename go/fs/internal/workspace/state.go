package workspace

import "context"

// StateEngine manages the physical storage and isolation of the workspace.
type StateEngine interface {
	// InitThread establishes the base state for a new conversational thread.
	InitThread(ctx context.Context, mainDir string, threadID string) error

	// Snapshot permanently saves the accepted state of the main thread.
	Snapshot(ctx context.Context, mainDir string, threadID string, message string) (string, error)

	// --- Agentic Sandbox Management ---

	// SpawnSandbox creates a physically isolated environment for a sub-agent.
	// It returns the absolute path to this new temporary directory.
	SpawnSandbox(ctx context.Context, mainDir string, threadID string, candidateID string) (sandboxDir string, err error)

	// CommitSandbox saves a checkpoint of the agent's work within its isolated environment.
	// Agents can call this multiple times to record their trial-and-error process.
	CommitSandbox(ctx context.Context, sandboxDir string, message string) (string, error)

	// SubmitSandbox syncs the agent's isolated candidate branch back to the central thread repository.
	SubmitSandbox(ctx context.Context, sandboxDir string, mainDir string, candidateID string) error

	// CloseSandbox securely deletes the physical temporary directory, leaving only the synced Git history.
	CloseSandbox(ctx context.Context, sandboxDir string) error

	// --- Main Session / User Review ---

	// PreviewCandidate temporarily moves the main working directory to view a submitted candidate.
	PreviewCandidate(ctx context.Context, mainDir string, threadID string, candidateID string) error

	// Accept fast-forwards the thread to include the candidate, logging the reason.
	Accept(ctx context.Context, mainDir string, threadID string, candidateID string, reason string) error

	// Reject cleans up the candidate branch from the active workspace, leaving an annotated tag with the reason.
	Reject(ctx context.Context, mainDir string, threadID string, candidateID string, reason string) error

	ReadCandidateDiff(ctx context.Context, repoRoot, threadID, candidateID string) (string, error)
}
