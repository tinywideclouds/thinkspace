# ThinkGraph Architecture

The `thinkgraph` package manages the dual-write cognitive memory and materialized state of the ThinkSpace system. It strictly separates the immutable audit trail (the Ledger) from the high-speed read index (the Graph) using a CQRS (Command Query Responsibility Segregation) pattern, ensuring strong typing and distinct consumer views.

## Core Concepts

**Event (Write Model):** An immutable storage envelope defining a single occurrence in time. It holds strongly typed domain payloads (`Prompt`, `ModelResponse`, `BranchAnchor`, `Digest`) within its `Data` field, remaining strictly decoupled from visual or contextual presentation.

**Node (Read Model):** The stateful read-model envelope stored in a Graph (primary implementation uses BadgerDB). It wraps the core `Event` and introduces `MaterializedState`—a projection data cache holding heavy, enriched artifacts (like parsed Git diffs or flow states) generated once during ingestion to prevent repetitive I/O on every read.

## The Dispatch Pipeline

Write operations are coordinated by the **Dispatcher**, which multiplexes incoming payloads through a unified `EventWriter` interface to two distinct storage layers:

1. **The Ledger (`FileLedger`):** The absolute source of truth. The dispatcher appends the pure, uninflated `Event` into durable, sharded `.jsonl` files on disk (`logs/`), alongside Markdown frontmatter files for `digests/` and `contexts/`, and JSON records for `mutations/`.
2. **The Graph (`GraphStore`):** The fast-read index. `GraphStore` intercepts the `Event`, routes it through `GraphBuilder` to inflate external dependencies (caching them in `Node.MaterializedState`), and persists the resulting `Node` into BadgerDB.

## Rehydration and State Recovery

Because the Graph is a materialized view, it can be entirely rebuilt from the Ledger. The **Rehydrator** engine validates synchronization state on initialization.

If the BadgerDB index is wiped or falls behind the physical disk history, the Rehydrator streams the append-only `.jsonl` Ledger events, replays digests, space contexts, and mutations, and repopulates the Graph. This guarantees parity between live WS traffic and offline recovery.

## Consumer Projections

The Graph acts as a single, unified read-model serving different audiences without bloating nodes with consumer-specific logic:

* **The UI Projections (in API Facade):** Fetches `Nodes`, extracts `MaterializedState`, and maps them into outbound Protobufs for timeline and graph exploration views.
* **The LLM Projection (`ContextProjecter`):** Projects a focused context window (`LongTermMemory`, collapsed `WorkingMemory`, inline amendments, and `Catalyst`) into a `cognition.Trajectory` for `ThinkBundle` assembly.