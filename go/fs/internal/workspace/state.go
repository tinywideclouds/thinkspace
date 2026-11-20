package workspace

import "context"

// ChatVersioner manages the long-term, permanent history of conversational threads. This is not git - we use git as an implementation detail, but the interface abstracts away the underlying VCS. It is responsible for initializing new chats, managing candidate sandboxes, and handling the acceptance or rejection of proposed changes.
type ChatVersioner interface {
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

// FileReader provides read-only file access within a workspace or sandbox.
type FileReader interface {
	// ReadFile reads a file from the virtual workspace.
	ReadFile(ctx context.Context, path string) ([]byte, error)
}

// FileWorkspace provides file read/write and path-resolution capabilities without exposing command execution or VCS lifecycle controls.
type FileWorkspace interface {
	FileReader

	// ResolvePath maps a workspace-relative path (e.g., "src/main.go") to its physical location on disk.
	ResolvePath(path string) string

	// WriteFile writes a file to the virtual workspace.
	WriteFile(ctx context.Context, path string, data []byte) error
}

// CommandRunner executes external commands (e.g., compilers, test runners) within an isolated workspace.
type CommandRunner interface {
	// ExecuteCommand runs a command securely within the workspace's context.
	ExecuteCommand(ctx context.Context, command string, args ...string) (string, error)
}

// CandidateSandbox represents a short-lived, isolated workspace for an agent to draft changes.
// It composes FileWorkspace and CommandRunner with candidate VCS lifecycle operations.
type CandidateSandbox interface {
	FileWorkspace
	CommandRunner

	// ApplyDraft commits the current virtual state of the sandbox to its local isolated history.
	ApplyDraft(ctx context.Context, message string) error

	// DeliverForReview submits the agent's final isolated work back to the central thread repository.
	DeliverForReview(ctx context.Context) error

	// TearDown securely deletes or cleans up the virtual environment.
	TearDown(ctx context.Context) error
}
