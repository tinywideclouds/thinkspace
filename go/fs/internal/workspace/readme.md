# Workspace Domain Layer

This package represents the core domain logic of the Thinkspace application. Its primary intention is to decouple the **Active Chat Logic** (the human-AI conversational loop) from the **File Logic** (the version control and file storage mechanics)[cite: 29].

By enforcing this boundary, the application can safely record streaming thoughts and tool calls without tangling with or corrupting the underlying sandbox environments[cite: 29].

## Core Concepts

The domain models a "Two-Speed" architecture that separates accepted reality from unverified ideas[cite: 29].

* **Threads (The Slow Path):** A `Thread` represents our accepted ledger of reality. It tracks the permanent conversation history and the verified codebase[cite: 28, 29].
* **Candidates (The Fast Path):** A `Candidate` represents unverified tool outputs. It acts as a temporary, isolated sandbox where the AI can propose code changes without impacting the Thread[cite: 28, 29].
* **The Ledger:** The conversational state is maintained in an append-only `conversation.jsonl` file. This acts as a Write-Ahead Log (WAL) that safely records events like prompts, model outputs, and candidate resolutions in real-time[cite: 28, 29].
* **Shadow Ledgers:** During concurrent sub-agent execution, intermediate thoughts are written to a `trace.jsonl` file in the sandbox. This file is safely extracted and copied to the main thread directory before the temporary sandbox is destroyed[cite: 24].

## Orchestration and Concurrency

The `workspace` package is designed to handle multiple AI agents operating simultaneously.

* **Mutex Lock Safety:** The `FanOutFlow` utilizes a `sync.Mutex` (`stateMu`) to strictly sequence operations on the `StateEngine` (e.g., spawning sandboxes, committing files, and submitting branches)[cite: 24]. This prevents Git lock collisions (`.git/index.lock`) while allowing the high-latency LLM network calls and domain verifications to run completely in parallel[cite: 24].
* **Token Multiplexing:** Sub-agent executions accept a `chan<- AgentToken`[cite: 26]. This channel safely routes multiple concurrent LLM token streams back to the UI, tagged with their originating `AgentID`[cite: 24, 26].

## Context Hierarchy & Timeouts

To prevent the system from hanging due to AI hallucinations (e.g., generating infinite loops in unit tests), the domain enforces a strict context timeout hierarchy[cite: 27, 32]:

1. **Turn Timeout (`TurnTimeoutSeconds`):** Bounds the entire manager orchestration loop (default: 5 minutes)[cite: 32].
2. **Agent Timeout (`AgentTimeoutSeconds`):** Bounds the specific generation phase of a single sub-agent (default: 1 minute)[cite: 32].
3. **Verify Timeout (`VerifyTimeoutSeconds`):** A strict local timeout applied directly to OS-level `exec.Command` calls like `go build` and `go test` (default: 15 seconds)[cite: 27, 32].

## The StateEngine Abstraction

The `workspace.Service` orchestrates the lifecycle using the `StateEngine` interface[cite: 29, 30]. The Service relies entirely on this abstraction, interacting strictly in domain terms rather than infrastructure commands[cite: 29, 30].

The application mandates the following outcomes from the engine[cite: 29, 31]:

* **`InitThread`:** Establishes the base state for a new conversational thread[cite: 29, 31].
* **`Propose`:** Creates an isolated sandbox, writes the generated code, saves it permanently for reference, and leaves the workspace safely on this sandbox for human review[cite: 29, 30].
* **`Accept`:** Integrates a proposed set of changes into the thread's accepted reality[cite: 29, 31].
* **`Reject`:** Cleans up the active sandbox for a proposal while leaving the data accessible via permanent storage mechanisms[cite: 29, 31].
* **`Snapshot`:** Permanently saves the accepted state of the thread, including both the code and the ledger[cite: 29, 31].