### Architectural Note: The "Floating Ledger" and Engine Implementations

**The Domain Requirement**
Our chat orchestration relies on a "floating ledger" pattern. Uncommitted state—specifically the `conversation.jsonl` file and other transient artifacts—must survive branch transitions. When the engine switches between the thread branch and a candidate branch (during `Preview`, `Accept`, or `Reject`), these untracked or modified files must float seamlessly into the new working directory state without causing conflicts or being overwritten.

**Sandbox Path Normalization**
To support both "Shared Codebase" and "Jailed" workflows without polluting the domain's `workspace.CandidateSandbox` interface, the concrete engines implement a `normalizePath` router. The `SharedCodebase` boolean flag is passed at engine instantiation and cascades down to the sandboxes. When an agent requests to read/write `src/main.go`, the router determines whether to anchor that path to the repository root or the isolated `chats/<chat-id>/` directory dynamically.

**The Native Exec Implementation (`GoExecChat`)**
The `GoExecChat` engine wraps the standard OS-level Git CLI. This engine handles the floating ledger gracefully and natively. Standard commands like `git checkout <branch>` automatically evaluate the index; if an untracked or modified file does not conflict with the target branch, the CLI safely carries it over while perfectly extracting the target branch's tracked files (like `api.go`) to the physical disk. It is the gold standard for how this orchestration should behave.

**The `go-git` Implementation (`GoGitChat`) & The Explicit Stash Hack**
In the `GoGitChat` engine, I (the AI) struggled to elegantly replicate that native CLI checkout behavior using the provided `CheckoutOptions`. To definitively satisfy the domain requirement and ensure tests pass, I implemented a brute-force wrapper called `withFloatingLedger`.

This wrapper works by:

1. Scanning the worktree status and reading all `Modified` and `Untracked` files into memory.
2. Executing a `Force: true` checkout to mathematically guarantee the target branch's files are materialized onto the physical disk.
3. Writing the stashed ledger files back to the filesystem.

**⚠️ Warning to Future Developers (and AIs)**
This in-memory stashing mechanism is a heavy-handed hack to force the desired outcome. It is not an indictment of the `go-git` library, but rather a limitation of my current ability to navigate its index and worktree API for this specific non-bare push/checkout edge case.

We do not like this hack. If you know the idiomatic, elegant way to configure `go-git` to natively float unstaged changes while successfully extracting newly pushed candidate files, **please replace `withFloatingLedger` immediately**.