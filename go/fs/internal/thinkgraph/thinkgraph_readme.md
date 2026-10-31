# ThinkGraph Architecture

The `thinkgraph` package manages the dual-write cognitive memory and materialized state of the ThinkSpace system. It strictly separates the immutable audit trail (the Ledger) from the high-speed read index (the Graph) using a CQRS (Command Query Responsibility Segregation) pattern, ensuring strong typing and distinct consumer views.

## Core Concepts

**Event (Write Model):** An immutable storage envelope defining a single occurrence in time. It holds strongly typed domain payloads (`Prompt`, `ModelResponse`, `CandidateProposal`, `Digest`) within its `Data` field, remaining strictly decoupled from visual or contextual presentation.

**Node (Read Model):** The stateful read-model envelope stored in a Graph (primary implementation is using BadgerDB). It wraps the core `Event` and introduces `MaterializedState`—a projection data cache holding heavy, enriched artifacts (like parsed Git diffs) generated once during ingestion to prevent repetitive I/O on every read.

## The Dispatch Pipeline

Write operations are coordinated by the **Dispatcher**, which multiplexes incoming payloads through a unified `EventWriter` interface to two distinct storage layers:

1. **The Ledger (`FileLedger`):** The absolute source of truth. The dispatcher appends the pure, uninflated `Event` into highly durable, sharded `.jsonl` files on disk.
2. **The Graph (`GraphStore`):** The fast-read index. The `GraphStore` intercepts the `Event`, routes it through the `GraphBuilder` to inflate external dependencies (caching them in the `Node.MaterializedState`), and persists the resulting `Node` into BadgerDB.

## Rehydration and State Recovery

Because the Graph is a materialized view, it can be entirely rebuilt from the Ledger. The **Rehydrator** engine validates synchronization state on initialization.

If the BadgerDB index is wiped or falls behind the physical Git disk history, the Rehydrator streams the append-only `.jsonl` Ledger events, passes them through the exact same `GraphBuilder` ingestion pipeline, and repopulates the Graph. This guarantees mathematical parity between live WS traffic and offline recovery.

## Consumer Projections

The Graph acts as a single, unified read-model allows it to serve different audiences. Rather than bloating the graph with consumer-specific logic, external projectors flatten the `Node` based on their requirements:

* **The UI Projections (in API Facade):** The frontend requires heavy visual context. The API layer fetches `Nodes`, extracts the enriched `MaterializedState` (such as code snippets or structural diff blocks), and maps it into outbound Protobufs for terminal or browser rendering. We currently target 2 UI projections: a timeline (which supports tree branching) and a graph exploration projection allowing a full traversal of the Graph.
* **The LLM Projection (`ContextProjecter`):** The cognitive engine requires lean, semantic context to protect its token window. The `ContextProjecter` fetches from the same underlying graph but targets a focused context into a `ThinkBundle`. Thinkbundles are <i>Thinkstate<i> aware, so for a golang prompt the ThinkBundle could contain an AST skeleton of the workspace with fully materialized files that are being currently worked on, along with a description of the current task.