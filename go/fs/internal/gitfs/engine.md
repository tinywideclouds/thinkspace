# GitFS State Engine

The `gitfs` package provides file state and version control management for ThinkSpace.

The architecture is built around the `workspace.ChatVersioner` interface (`SpawnCandidateSandbox`, `PreviewCandidate`, `ReadCandidateDiff`, `Accept`, `Reject`) so the underlying Git implementation can be swapped without affecting session or workspace orchestration.

## Supported Engines

### 1. GoExec Engine (`GoExecEngine`)
Shells out to the native OS-level Git CLI via `os/exec` and uses `git worktree` for fast candidate sandbox provisioning and native untracked/modified file floating during branch switches.

### 2. GoGit Engine (`GoGitEngine`)
Uses pure Go `github.com/go-git/go-git/v5` (`git.PlainCloneContext` for sandboxes and `withFloatingLedger` to preserve modified/untracked ledger files across forced checkouts) with zero external Git binary dependency.

## Sandbox Execution Modes

* **Shared Codebase Mode (`sharedCodebase: true`):** Sandboxes root at the workspace directory so agents edit shared project files directly (`src/...`).
* **Jailed Mode (`sharedCodebase: false`):** Sandboxes isolate file operations inside `chats/<chat-id>/`.

Both engines enforce sandbox root containment (`resolveSafePath`) and synchronize repository-level Git metadata mutations so concurrent fan-out sub-agents never collide on Git index/worktree locks.