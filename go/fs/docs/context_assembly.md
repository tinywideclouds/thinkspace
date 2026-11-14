# Context Assembly Architecture

Context assembly in ThinkSpace transforms persisted graph history, long-term memory, and live workspace state into a structured `cognition.ThinkBundle` for the LLM—and projects that bundle into a `session.TurnContext` when human-in-the-loop (JIT) context review is active[cite: 1].

## Architectural Split

Context assembly is intentionally split across two packages to separate **orchestration/I/O** from **pure graph projection**[cite: 1]:

| Component | Package | Responsibility |
| :--- | :--- | :--- |
| **`BundleAssembler`** | `internal/session` | Performs graph/disk I/O, resolves the active `Catalyst`, filters digests and space contexts by focus tags, generates workspace `Mapbook` layers, renders the persona template, and enriches the bundle via `ThinkState`[cite: 1]. |
| **`ContextProjecter`** | `internal/thinkgraph` | Pure, stateless projection from gathered graph nodes (`ContextRequest`) into a valid `cognition.Trajectory` (`LongTermMemory`, `WorkingMemory`, `Catalyst`)[cite: 1]. |

---

## Step-by-Step Assembly Pipeline

When `ChatEngine.runStandard` starts a turn, it invokes `BundleAssembler.Assemble(...)` with the active `ChatAddress`, `digestionHorizon`, `catalystID`, and `ThinkState`[cite: 1]:

### 1. Reverse Timeline Traversal & Tag Focus (`BundleAssembler.gatherTimeline`)
* Uses `thinkgraph.TraversalEngine.WalkTimelineReverse` to walk backwards from the active branch head (crossing `EventBranchAnchor` boundaries into parent branches automatically)[cite: 1].
* Halts when it reaches `digestionHorizon` or the 50-event window cap[cite: 1].
* Classifies node tags by recency[cite: 1]:
  * Events within `FocusDepth` (default `3`) populate `PrimaryFocus` tags[cite: 1].
  * Older events in the window populate `RelatedContext` tags[cite: 1].
* Reverses the collected slice into chronological order[cite: 1].

### 2. Long-Term Memory Filtering (`BundleAssembler.Assemble`)
* Queries `GetStickyDigests` (promoted digests) and `GetSpaceContexts` from `GraphRepository`[cite: 1].
* Keeps items that either have **no tags** (global rules/anchors) or share at least one tag with `PrimaryFocus` or `RelatedContext`[cite: 1].

### 3. Catalyst Resolution & Deduplication
* Looks up the explicit `catalystID` node (or falls back to the tail node of the timeline if `catalystID` is nil)[cite: 1].
* Strips `catalystNode` out of `volatileEvents` (`RecentEvents`) so the current prompt is never duplicated in `WorkingMemory`[cite: 1].

### 4. Trajectory Projection (`thinkgraph.ContextProjecter.Build`)
`ContextProjecter` converts the `ContextRequest` into a `cognition.Trajectory`[cite: 1]:
1. **Long-Term Memory:** Prepends a `[CONTEXT HORIZON METADATA]` block listing `PrimaryFocus`, `RelatedContext`, and `DormantLenses`, followed by active `SpaceContexts` and promoted `Digests`[cite: 1].
2. **Working Memory:** Iterates through `RecentEvents`[cite: 1]:
   * Skips `EventSystem` nodes[cite: 1].
   * Maps `EventModel` to role `"model"` and user/flow events to role `"user"`[cite: 1].
   * Injects inline `[System Note: This prompt was later amended to: ...]` annotations if `amended_by` metadata is present[cite: 1].
   * Collapses consecutive events of the same role into a single turn[cite: 1].
3. **Catalyst Binding:**
   * Requires a non-nil `Catalyst` with non-empty content (returning an explicit error otherwise)[cite: 1].
   * If `WorkingMemory` ends with the same role as `Catalyst` (for example, a persisted user decision node like `accepted flow: ...` immediately preceding the next user prompt), `ContextProjecter` pops that trailing entry off `WorkingMemory` and merges it into `Catalyst`[cite: 1]. This guarantees `WorkingMemory` alternates cleanly and `Catalyst` is always populated[cite: 1].

### 5. Mapbook, Persona & Domain Enrichment
* **Mapbook:** Calls `thinkState.Mapbook().GenerateLayers(...)` against `{workspaceRoot}/{projectID}/workspace`[cite: 1].
* **Persona:** Renders `ManagerSystemPrompt` as a Go `text/template` with `{{.AvailableThinkStates}}` and appends the active `ThinkState` and taxonomy rules[cite: 1].
* **Enrichment:** Calls `thinkState.EnrichBundle(...)` so language-specific `ThinkState` implementations can inject structural AST skeletons or targeted workbench files[cite: 1].

---

## JIT Context Review & Serialization

1. **Projection (`ProjectTurnContext`):** Projects `ThinkBundle` into `session.TurnContext`, keeping `RecentHistory` (`WorkingMemory`) and `Catalyst` (the active prompt) strictly separate so the UI displays `Catalyst` as read-only while allowing the user to toggle individual `WorkingMemory` turns and `Workbench` files[cite: 1].
2. **Pruning (`ChatEngine.contextReview`):** Applies `RetainedFilePaths` and `RetainedHistoryIndices` to `bundle.Workbench` and `bundle.Trajectory.WorkingMemory`[cite: 1]. An empty `WorkingMemory` slice is valid; `Catalyst` always remains intact[cite: 1].
3. **Gemini Serialization (`gemini.Adapter.compileHistory`):** Emits `LongTermMemory` pairs, `WorkingMemory` turns, and finally `Workbench` + `Catalyst` as the closing user turn[cite: 1].