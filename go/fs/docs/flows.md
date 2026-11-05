# Flow example 

Looking at the `ContextProjecter` in the codebase, we have specific semantic events for this exact lifecycle: `"candidate_proposal"` and `"candidate_resolution"`.

Here is exactly how both scenarios flow through the Dispatcher:

**1. The Proposals (The Divergence)**
Two sub-LLMs generate competing code. The workspace creates two Git sandboxes.

* **The Action:** The backend orchestrator emits two `"candidate_proposal"` events.


* **Ledger:** The Dispatcher appends two lightweight JSON lines, simply recording the event and the candidate IDs (Git references).


* **Graph:** The `GraphBuilder` intercepts the events, reaches into the workspace using the candidate IDs to fetch the physical diffs, and caches those diffs directly in the `Node.MaterializedState`.



**2. The Decision (Human vs. Agentic)**

* **Human-in-the-loop:** The Angular UI receives the Graph Nodes via WebSocket, rendering the diffs from the `MaterializedState`. The user clicks "Accept A". The API translates this into a `"candidate_resolution"` event.


* **Fully Agentic:** A Manager LLM evaluates both sandboxes (perhaps using the `query_lens` or `propose_change` tools) and decides "Candidate A is better." The orchestrator translates this into the exact same `"candidate_resolution"` event.



**3. The Dispatch & Mutation**

* **The Event:** `Dispatcher.AppendEvent` routes the `"candidate_resolution"` to the Ledger (appending the choice) and the Graph (creating the semantic node).


* **The Flow Update:** The active Flow node is then updated via `Dispatcher.SaveMutation` to reflect that this milestone is complete. The Ledger appends this mutation delta to a file in `ledger/mutations`, while the Graph Store applies the update to the flow node's `MaterializedState` in BadgerDB.
