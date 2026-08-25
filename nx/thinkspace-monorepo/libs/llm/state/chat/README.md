# llm-state-chat

This library acts as the central "brain" of the ThinkSpace web UI. It maps domain events to reactive Angular Signals, maintaining the state machine of the multi-agent CLI loop.

## Responsibilities
* Connect to the `TransportService` and route domain events through the `LlmFacade`.
* Manage partitioned UI state using modern Angular Signals (`coreChat`, `systemLogs`, `activeAgents`).
* Handle blocking workflow states (`pendingStrategy`, `pendingReviewBranch`) replicating the synchronous terminal experience.

## Rules
* **MUST** use `inject()` for all dependency resolution.
* **MUST NOT** include any HTML templates or UI rendering logic.
* Operates exclusively on Domain types imported from `@org/llm-core-facade`.