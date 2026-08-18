# Workspace Domain Layer

This package represents the core domain logic of the Thinkspace application. Its primary intention is to decouple the **Active Chat Logic** (the human-AI conversational loop) from the **File Logic** (the version control and file storage mechanics).

By enforcing this boundary, the application can safely record streaming thoughts and tool calls without tangling with or corrupting the underlying sandbox environments.

## Core Concepts

The domain models a "Two-Speed" architecture that separates accepted reality from unverified ideas.

* **Threads (The Slow Path):** A `Thread` represents our accepted ledger of reality. It tracks the permanent conversation history and the verified codebase.


* **Candidates (The Fast Path):** A `Candidate` represents unverified tool outputs. It acts as a temporary, isolated sandbox where the AI can propose code changes without impacting the Thread.


* **The Ledger:** The conversational state is maintained in an append-only `conversation.jsonl` file. This acts as a Write-Ahead Log (WAL) that safely records events like prompts, model outputs, and candidate resolutions in real-time.



## The StateEngine Abstraction

The `workspace.Service` orchestrates the lifecycle using the `StateEngine` interface. The Service relies entirely on this abstraction, interacting strictly in domain terms rather than infrastructure commands.

The application mandates the following outcomes from the engine:

* **`InitThread`:** Establishes the base state for a new conversational thread.


* **`Propose`:** Creates an isolated sandbox, writes the generated code, saves it permanently for reference, and leaves the workspace safely on this sandbox for human review.


* **`Accept`:** Integrates a proposed set of changes into the thread's accepted reality.


* **`Reject`:** Cleans up the active sandbox for a proposal while leaving the data accessible via permanent storage mechanisms.


* **`Snapshot`:** Permanently saves the accepted state of the thread, including both the code and the ledger.



## The Lifecycle Loop

1. **Prompting:** The user submits a prompt, which the Service immediately logs to the ledger.


2. **Proposing:** When the AI triggers a tool call, the Service delegates to the `StateEngine` to propose a candidate. The engine isolates the generated files, while the uncommitted ledger securely "floats" alongside it.


3. **Reviewing:** The application pauses, allowing the human to inspect the isolated sandbox.
4. **Resolving:** The user accepts or rejects the candidate. The Service logs this resolution to the ledger and commands the engine to either integrate the code or clean up the sandbox.


5. **Checkpointing:** At logical breakpoints, the Service takes a snapshot, permanently committing the Thread's state.