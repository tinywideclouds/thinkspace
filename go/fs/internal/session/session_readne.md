# Session Orchestration Layer

The `session` package serves as the central brain of the ThinkSpace application. It orchestrates the high-level human-AI interaction loop by connecting the LLM manager, the workspace domain, and the user interface.

## Core Components

* **`Manager`:** The top-level singleton injected into the API server. It caches active `ChatSession` instances and provides the entrypoint for executing turns (`RunTurn`).
* **`ChatSession`:** Represents the active state of a single conversation thread. It manages metadata locks, resolves branch `GraphID`s, initializes chat boundaries, and drives the signal-routing loop across specialized `TurnEngine` implementations.
* **`BundleAssembler`:** Walks the BadgerDB graph backwards, filters active sticky digests and space rules, maps the physical directory tree, and formats the persona template into a `ThinkBundle`.
* **`TurnExecutor`:** Manages LLM streaming during a turn, persists model responses, intercepts tool calls (`query_lens`, `propose_change`, `transition_workspace`, `create_digest`), and routes control signals back to `ChatSession`.
* **`WorkerEngine` & `SubAgentRunner`:** Execute configured flows (`FanOutFlow`, `ResearchFlow`), spawning concurrent sub-agents in isolated candidate sandboxes, running CI verification retries, and collecting user strategy choices.
* **`SynthesizeEngine`:** Orchestrates multi-stage synthesis (`refine_prompt` Manager evaluation on the flow subgraph $\rightarrow$ `synthesize_prompt` Worker patch generation $\rightarrow$ `synthesis_review_prompt` Manager final evaluation on the main timeline $\rightarrow$ user Accept/Reject resolution).
* **`SystemEngine`:** Executes ThinkState domain transitions, initializes new domain directories, appends system audit events to the `system` subgraph, and reboots the turn.
* **`UserInterface`:** Abstracts the frontend so the cognitive engines operate identically across CLI and WebSocket clients.

## Delegation Strategies

When sub-agents propose code changes via a Fan-Out Flow, the user selects one of four strategies:
* **Skip (`StrategySkip`):** Automatically rejects all candidate branches and continues the chat.
* **Manual (`StrategyManual`):** The user manually previews and accepts/rejects candidate branches without synthesis.
* **Review (`StrategyReview`):** Manual review of candidate branches.
* **Refine (`StrategyRefine`):** Handed off to `SynthesizeEngine` to evaluate the candidate branches, synthesize a unified solution, present a final evaluation on the main timeline, and prompt for Accept/Reject.

## The Manager Subgraph: Fan-Out Diamond Topology

`WorkerEngine` executes parallel agent workflows using a "Diamond" DAG topology:

1. **Origin (`main`):** The Manager's ephemeral response node (`turnID`) acts as the structural head of the workflow (`parent_id` $\rightarrow$ `catalystID`).
2. **Flow Card (`main`):** `EventFlowLifecycle` (`flowID`, `parent_id` $\rightarrow$ `turnID`) tracks live execution state.
3. **Subgraph Fork (`flow-<uuid>`):** $N$ sub-agent `EventPrompt` nodes link cross-boundary (`parent_id` $\rightarrow$ `turnID`).
4. **Parallel Execution (`flow-<uuid>`):** Each agent produces `EventModel` and `EventCandidateResolution` nodes inside its isolated sandbox.
5. **Subgraph Join (`flow-<uuid>`):** `joinNodeID` joins all leaf candidate nodes (`parent_0`...`parent_N`).
6. **Yield to Main (`main`):** `optionsNodeID` (`EventSystem`, `parent_id` $\rightarrow$ `joinNodeID`) yields control to the user.
7. **Synthesis Continuation (`flow-<uuid>` $\rightarrow$ `main`):** On `StrategyRefine`, `SynthesizeEngine` runs Stage 1 (Manager comparative evaluation) and Stage 2 (Synthesis worker patch) on `flow-<uuid>`, then runs Stage 3 (Manager final review) and Stage 4 (user Accept/Reject) on `main`.