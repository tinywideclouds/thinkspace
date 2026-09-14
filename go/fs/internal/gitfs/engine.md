# GitFS State Engine

The `gitfs` package provides file state and version control management for ThinkSpace.

Instead of hardcoding a single Git strategy, the architecture is built around a pluggable `workspace.ChatEngine` interface. This interface defines the strict lifecycle contract (Spawn, Preview, Accept, Reject) required to orchestrate isolated agents, while allowing the underlying Git implementation to be swapped dynamically based on environment or performance requirements.

## Supported Engines

The package currently provides two concrete implementations of the `workspace.ChatEngine` interface:

### 1. GoExec Engine (`GoExecEngine`)
This implementation shells out to the native OS-level Git CLI via `os/exec`.
* **Mechanics:** It heavily leverages native `git worktree` commands to instantly provision sandboxes linked to the main repository database.
* **Strengths:** It is the "gold standard" implementation. It handles complex worktree operations, untracked folder staging, and "floating" uncommitted ledgers seamlessly because it relies on the highly optimized, native Git binary.

### 2. GoGit Engine (`GoGitEngine`)
This implementation uses the pure Go `github.com/go-git/go-git/v5` library.
* **Mechanics:** It relies on in-memory object traversal and traditional `git clone` commands to provision sandboxes.
* **Strengths:** It is highly portable and requires no external system dependencies (like an installed Git binary).
* **Limitations:** Because `go-git` lacks native support for floating unstaged changes during complex branch checkouts, this engine relies on an in-memory stashing mechanism (`withFloatingLedger`) to safely carry the `conversation.jsonl` file across branch transitions.

---

## Sandbox Execution Modes

Regardless of which engine is used, the system supports two distinct execution models for how agents interact with the repository. This is governed by the `SharedCodebase` flag passed during engine initialization:

*   **Shared Codebase Mode (Same Developer):** The sandbox targets the repository root. Agents edit the shared application code directly (e.g., `/src`). Git handles conflict resolution naturally when the chat branch is eventually merged back to `main`.
*   **Jailed Mode (Separate Developer):** The sandbox intercepts file paths and traps them inside the metadata folder (e.g., `/chats/<chat-id>/src`). Agents believe they are writing to the root, but the engine physically isolates the code to prevent all structural collisions.

*Note:* In both modes, the `conversation.jsonl` ledger is strictly stored in the `chats/<chat-id>/` directory to prevent metadata conflicts.

## The Lifecycle Methods

Both engines strictly adhere to the following lifecycle:

### 1. Propose (`SpawnCandidateSandbox` & `ApplyDraft`)
* The engine creates a candidate branch and spawns an isolated execution sandbox.
* The agent writes files to this sandbox, and `ApplyDraft` commits them to the candidate branch.
* **Crucially:** The sandbox allows external tools (like a user's IDE) or test runners to inspect and verify the files before a final decision is made.

### 2. Read Candidate Context (`ReadCandidateDiff`)
* This method allows the LLM Manager to review the agent's proposed changes by generating a triple-dot diff (`git diff main...candidate`).
* By comparing the target thread branch tree against the candidate branch tree, it outputs a standard Unified Format Patch showing exact additions (`+`) and deletions (`-`), which is ideal for LLM context processing.

### 3. Accept (`Accept`)
Accepting a candidate executes a fast-forward merge:
1. **Checkout Thread:** The engine safely checks out the main thread branch, natively floating the ledger.
2. **Fast-Forward:** The thread's branch reference is updated to point to the candidate's commit hash.
3. **Sync Worktree:** The working directory is synced to extract the accepted files permanently to the disk.
4. **Tagging:** An annotated tag (`refs/tags/proposal-<id>`) is created permanently marking the accepted code.
5. **Cleanup:** The candidate branch reference is destroyed.

### 4. Reject (`Reject`)
* Rejection is handled with a single, safe checkout back to the main thread branch.
* Because the rejected files do not exist on the target thread branch, Git natively deletes them from the working directory.
* An annotated tag is created permanently recording the rejection reason.
* The candidate branch reference is cleanly deleted from the storer.

## Verification

The engines are validated by a shared, black-box contract test suite (`engine_contract_test.go`). The suite iterates over both `GoGitEngine` and `GoExecEngine` factories, passing them through identical lifecycle simulations. This ensures that regardless of the underlying Git mechanics, both engines perfectly respect the `SharedCodebase` routing flag, float uncommitted ledgers without data loss, and enforce the exact same filesystem state upon acceptance or rejection.