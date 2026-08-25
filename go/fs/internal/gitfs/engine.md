# GitFS State Engine

The `gitfs` package provides the `NativeEngine`, which implements the `workspace.StateEngine` interface using pure Go via the `[github.com/go-git/go-git/v5](https://github.com/go-git/go-git/v5)` library.

This implementation is specifically designed to leverage native Git behaviors to manage file state without interfering with the active, uncommitted chat ledger.

## Implementation Mechanics

The engine relies on standard, safe Git checkouts to manage the application's "Two-Speed" architecture.

### The "Floating" Ledger

The core requirement of this engine is that uncommitted modifications to the `conversation.jsonl` file must never be destroyed during tool execution.

* To achieve this, the engine strictly avoids the use of `Keep: true` or `Force: true` flags during branch transitions.[cite: 4]
* By using standard checkouts (`worktree.Checkout(&git.CheckoutOptions{Branch: target})`), Git natively "floats" uncommitted changes to tracked files across branches, provided those files do not have underlying commit conflicts.[cite: 4]



### Atomic Branching (`git switch -c`)

* When isolating a new proposal, the engine replicates the terminal command `git switch -c` by passing the starting commit hash alongside `Create: true` within the `CheckoutOptions`.[cite: 4]


* This atomically creates the new branch reference and switches the working directory to it, safely carrying the uncommitted ledger along.[cite: 4]



## The Lifecycle Methods

### 1. Propose (`Propose`)

* The engine creates a candidate branch and strictly stages only the newly proposed files in the `docs/` directory.[cite: 4]


* After committing, it attaches a permanent lightweight tag (e.g., `refs/tags/proposal-<id>`) so the code survives even if the branch is later deleted.[cite: 4]


* **Crucially:** The engine intentionally leaves the working tree checked out on the candidate branch. This allows external tools (like a user's IDE) to inspect the files before a decision is made.[cite: 4]

### 2. Read Candidate Context (`ReadCandidateDiff`)

* This method allows the LLM Manager to review the agent's proposed changes by generating a triple-dot diff (`git diff main...candidate`).
* By comparing the target thread branch tree against the candidate branch tree, it outputs a standard Unified Format Patch showing exact additions (`+`) and deletions (`-`), which is ideal for LLM context processing.

### 3. Accept (`Accept`)

Accepting a candidate executes a fast-forward merge using a three-step pointer manipulation:

1. **Checkout Thread:** The engine safely checks out the main thread branch, which natively floats the ledger and temporarily removes the candidate's files from disk.[cite: 4]


2. **Fast-Forward:** The thread's branch reference is manually updated in the storer to point to the candidate's commit hash.[cite: 4]


3. **Sync Worktree:** A second checkout of the thread branch forces Git to recognize the advanced HEAD pointer, extracting the accepted files permanently to the disk while continuing to float the ledger.[cite: 4]


4. **Cleanup:** The candidate branch reference is destroyed.[cite: 4]



### 4. Reject (`Reject`)

* Rejection is handled with a single, safe checkout back to the main thread branch.[cite: 4]


* Because the rejected files (e.g., `math.go`) do not exist on the target thread branch, Git natively deletes them from the working directory during the switch.[cite: 4]


* The candidate branch reference is then cleanly deleted from the storer, leaving no trace except the permanent tag.[cite: 4]



## Verification

The engine's behaviors are validated by a comprehensive black-box test suite (`engine_test.go`). The tests simulate a real-world application state by writing an uncommitted ledger to disk and verifying that it successfully survives both the `Accept` and `Reject` lifecycles without data loss. Furthermore, the tests guarantee that rejected files are completely erased from the disk, while accepted files persist.[cite: 4]