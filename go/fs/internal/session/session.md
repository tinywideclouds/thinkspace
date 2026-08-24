# Session Orchestration Layer

The `session` package serves as the central brain of the ThinkSpace application. It orchestrates the high-level human-AI interaction loop by connecting the LLM manager, the workspace domain, and the user interface[cite: 33].

## Core Components

* **`Coordinator`:** The primary struct responsible for managing a single conversational turn. It handles LLM streaming, intercepts tool calls, and dictates the workflow based on user decisions[cite: 33].
* **`UserInterface`:** An interface abstracting the frontend. This allows the `Coordinator` to operate identically whether it is communicating via a CLI (`TerminalUI`) or a Web Server (`WebSocketUI`)[cite: 33].
* **`DelegationStrategy`:** An enumeration defining how the system processes multiple sub-agent proposals[cite: 33].

## Delegation Strategies

When sub-agents propose code changes, the user can select one of four strategies[cite: 33]:

* **Skip (`StrategySkip`):** Automatically rejects all proposals and continues the chat[cite: 33].
* **Manual (`StrategyManual`):** The user manually previews and evaluates the candidate branches without AI assistance[cite: 33].
* **Review (`StrategyReview`):** The Manager LLM evaluates the candidate diffs, provides a comparative summary, and recommends an approach to the user[cite: 33].
* **Refine (`StrategyRefine`):** The Manager LLM evaluates the candidates and immediately spawns a new sub-agent to synthesize the ultimate best version based on its evaluation before asking for user approval[cite: 33].

## Turn Lifecycle

A single execution turn follows a strict sequence:

1. **Manager Generation:** The Manager LLM streams its response, which is routed directly to the UI[cite: 33].
2. **Tool Interception:** If the LLM requests a change, the `Coordinator` intercepts the `propose_change` tool call[cite: 33].
3. **Fan-Out Execution:** The `Coordinator` triggers the `FanOutFlow` to spawn concurrent sub-agents[cite: 33].
4. **Strategy Selection:** The UI prompts the user to select a `DelegationStrategy`[cite: 33].
5. **Evaluation:** If requested, the Manager LLM generates a comparative review of the diffs[cite: 33].
6. **Resolution:** The user previews the final candidates and explicitly accepts or rejects them[cite: 33].
7. **Checkpoint:** The `Coordinator` commands the workspace service to permanently snapshot the thread state[cite: 33].