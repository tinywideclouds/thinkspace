# Workspace Domain Layer

This package represents the core domain logic of the Thinkspace application. Its primary intention is to decouple the **Active Chat Logic** (the human-AI conversational loop) from the **File Logic** (the version control and file storage mechanics)[cite: 17].

By enforcing this boundary, the application can safely record streaming thoughts and tool calls without tangling with or corrupting the underlying sandbox environments[cite: 17].

## Core Concepts

The domain models a "Two-Speed" architecture that separates accepted reality from unverified ideas[cite: 17].

* **Threads (The Slow Path):** A `Thread` represents our accepted ledger of reality. It tracks the permanent conversation history and the verified codebase[cite: 17].
* **Candidates (The Fast Path):** A `Candidate` represents unverified tool outputs. It acts as a temporary, isolated virtual sandbox where the AI can propose code changes without impacting the Thread[cite: 17].
* **The Ledger:** The conversational state is maintained in an append-only `conversation.jsonl` file. This acts as a Write-Ahead Log (WAL) that safely records events like prompts, model outputs, and candidate resolutions in real-time[cite: 17].
* **Shadow Ledgers:** During concurrent sub-agent execution, intermediate thoughts are written to a `trace.jsonl` file in the sandbox. This file is safely extracted via the virtual environment before the temporary sandbox is destroyed[cite: 17].

## Orchestration and Concurrency

The `workspace` package is designed to handle multiple AI agents operating simultaneously[cite: 17].

* **Mutex Lock Safety:** The `FanOutFlow` utilizes a `sync.Mutex` to strictly sequence physical state transitions (e.g., spawning sandboxes, delivering branches)[cite: 17]. This prevents lock collisions while allowing high-latency LLM network calls and domain verifications to run completely in parallel[cite: 17].
* **Token Multiplexing:** Sub-agent executions accept a `chan<- AgentToken`[cite: 17]. This channel safely routes multiple concurrent LLM token streams back to the UI, tagged with their originating `AgentID`[cite: 17].

## Context Hierarchy & Timeouts

To prevent the system from hanging due to AI hallucinations, the domain enforces a strict context timeout hierarchy[cite: 17]:

1. **Turn Timeout:** Bounds the entire manager orchestration loop (default: 5 minutes)[cite: 17].
2. **Agent Timeout:** Bounds the specific generation phase of a single sub-agent (default: 1 minute)[cite: 17].
3. **Verify Timeout:** A strict local timeout applied directly to commands executing within the sandbox (default: 15 seconds)[cite: 17].

## The Factory & Sandbox Abstractions

The `workspace.Service` orchestrates the lifecycle using two purely domain-driven interfaces:

### `ChatEngine`
The factory and long-term history manager.
* **`InitChat`:** Establishes the base state for a new conversational thread.
* **`Snapshot`:** Permanently saves the accepted state of the thread.
* **`SpawnCandidateSandbox`:** Creates an isolated environment and returns it to the domain.
* **`Accept` / `Reject`:** Integrates or discards proposals from the main thread.

### `CandidateSandbox`
The ephemeral virtual environment for the sub-agent. The domain interacts with this sandbox purely via virtual I/O (`WriteFile`, `ReadFile`, `ExecuteCommand`), completely hiding physical disk paths.
* **`ApplyDraft`:** Commits the current virtual state to the sandbox's isolated history.
* **`DeliverForReview`:** Submits the finalized proposal back to the main session.
* **`TearDown`:** Securely destroys the virtual environment.