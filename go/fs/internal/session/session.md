# Session Orchestration Layer

The `session` package serves as the central brain of the ThinkSpace application. It orchestrates the high-level human-AI interaction loop by connecting the LLM manager, the workspace domain, and the user interface.

## Core Components

* **`TurnManager`:** The top-level orchestrator that executes a single conversational loop. It is responsible for initializing the workspace service, loading the historical state instantly from the BadgerDB graph, assembling the context prompt, and injecting the physical Mapbook before invoking the `Coordinator`.
* **`Coordinator`:** The primary struct responsible for managing the LLM execution stream during a turn. It intercepts tool calls and executes the recursive retrieval and branching logic.
* **`UserInterface`:** An interface abstracting the frontend. This allows the `Coordinator` to operate identically whether it is communicating via a CLI (`TerminalUI`) or a Web Server (`WebSocketUI`).
* **`FlowEngine`:** The generic execution engine that runs configured sequences of sub-agent tasks, handling the sandbox provisioning, UI reviews, and telemetry emissions.

## Delegation Strategies

When sub-agents propose code changes via a Fan-Out Flow, the user can select one of four strategies:
* **Skip (`StrategySkip`):** Automatically rejects all proposals and continues the chat.
* **Manual (`StrategyManual`):** The user manually previews and evaluates the candidate branches without AI assistance.
* **Review (`StrategyReview`):** The Manager LLM evaluates the candidate diffs, provides a comparative summary, and recommends an approach.
* **Refine (`StrategyRefine`):** The Manager LLM evaluates the candidates and immediately synthesizes an ultimate best version based on its evaluation.

## Turn Lifecycle

A single execution turn executed by the `Coordinator` follows a recursive loop (bounded to a maximum of 3 iterations):

1. **Manager Generation:** The Manager LLM streams its response based on the assembled context, which is routed directly to the UI.
2. **Tool Interception:** The `Coordinator` parses the model output for tool invocations.
    *   **Retrieval Augmentation (`query_lens`):** If the LLM requests more context about a specific semantic tag, the `Coordinator` queries the BadgerDB graph for the raw events matching that tag, injects the subgraph into the context array, and recursively loops back to Step 1.
    *   **Sub-Agent Delegation (`propose_change`):** If the LLM is ready to act, it calls `propose_change`, breaking the loop and initiating the fan-out sequence.
3. **Event-Driven Fan-Out:** The `Coordinator` triggers the `FlowEngine`. Sub-agents run concurrently in isolated virtual sandboxes, broadcasting structured lifecycle events in real-time.
4. **Verification & Strategy:** The CI verifier evaluates the code. The UI prompts the user to select a `DelegationStrategy`.
5. **Resolution:** The user previews the final candidates and explicitly accepts or rejects them.
6. **Checkpoint:** The `Coordinator` commands the workspace service to permanently snapshot the thread state in the Git layer.