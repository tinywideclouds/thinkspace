package workspace

import "context"

// ChatEngine manages the long-term, permanent Git history of conversational threads.
type ChatEngine interface {
	// InitChat establishes the base state for a new conversational thread.
	InitChat(ctx context.Context, chatID string) error

	// Snapshot permanently saves the accepted state of the main thread.
	Snapshot(ctx context.Context, chatID string, message string) (string, error)

	// SpawnCandidateSandbox creates a physically isolated environment for a sub-agent.
	SpawnCandidateSandbox(ctx context.Context, chatID string, candidateID string) (CandidateSandbox, error)

	// --- Main Session / User Review ---

	// PreviewCandidate temporarily moves the main working directory to view a submitted candidate.
	PreviewCandidate(ctx context.Context, chatID string, candidateID string) error

	// ReadCandidateDiff fetches the exact additions and deletions made by the candidate.
	ReadCandidateDiff(ctx context.Context, chatID string, candidateID string) (string, error)

	// Accept fast-forwards the thread to include the candidate, logging the reason.
	Accept(ctx context.Context, chatID string, candidateID string, reason string) error

	// Reject cleans up the candidate branch from the active workspace, leaving an annotated tag with the reason.
	Reject(ctx context.Context, chatID string, candidateID string, reason string) error
}

// CandidateSandbox represents a short-lived, isolated workspace for an agent to draft changes.
// It acts as a pure virtual environment, decoupling the domain from physical file paths.
type CandidateSandbox interface {
	// WriteFile writes a file to the sandbox's virtual workspace.
	WriteFile(ctx context.Context, path string, data []byte) error

	// ReadFile reads a file from the sandbox's virtual workspace.
	ReadFile(ctx context.Context, path string) ([]byte, error)

	// ExecuteCommand runs a command (e.g., linter, compiler) securely within the sandbox's context.
	ExecuteCommand(ctx context.Context, command string, args ...string) (string, error)

	// ApplyDraft commits the current virtual state of the sandbox to its local isolated history.
	ApplyDraft(ctx context.Context, message string) error

	// DeliverForReview submits the agent's final isolated work back to the central thread repository.
	DeliverForReview(ctx context.Context) error

	// TearDown securely deletes or cleans up the virtual environment.
	TearDown(ctx context.Context) error
}
