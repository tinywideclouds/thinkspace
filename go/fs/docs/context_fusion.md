# Context Assembly & Fusion

ThinkSpace utilizes a "Dual-Brain" architecture to construct the JIT (Just-In-Time) prompt for the Manager LLM. Because context windows are finite and expensive, the system strictly isolates the generation of *Temporal Memory* (what has happened) from *Spatial Reality* (what physically exists) before fusing them together at the orchestration layer.

## 1. Temporal Memory (`internal/chat`)
The Chat package is responsible for the Read Model projections of the immutable Ledger. The `chat.Assembler` translates raw historical events into conversational awareness.
*   **Sticky Context:** Heavily compressed summaries of older events marked as critical (e.g., overarching architecture decisions).
*   **Conceptual Tags:** An index of `#tags` that exist in the historical ledger, allowing the LLM to know what deep context is available to query via the `query_lens` tool.
*   **Recent Trajectory:** A strictly bounded sliding window (e.g., the last 5 events) showing the immediate conversational momentum.

*Crucially, the Chat layer has zero awareness of the physical file system.*

## 2. Spatial Reality (`internal/assembler`)
The Assembler package is responsible for the physical and environmental constraints of the workspace. It reads the disk and domain configuration.
*   **The Mapbook:** Provides the "Table of Contents" for the workspace. It yields the `LayerStructural` (the physical directory tree) and `LayerConceptual` (domain-specific rules, like "Go code must be in /src").
*   **The Workbench:** Extracts the exact, literal string contents of specific files when sub-agents or the Manager request them.

*Crucially, the Assembler layer has zero awareness of the chat history, users, or conversational momentum.*

## 3. Context Fusion (`internal/session`)
The Orchestration layer (specifically the `session.ContextAssembler` utilized by the `TurnManager` and `Coordinator`) acts as the master prompt compiler. 

Before invoking the LLM, the Fusion step:
1.  Requests the conversational baseline from the `chat.Assembler`.
2.  Requests the physical topology from the `Mapbook`.
3.  Weaves them together into the final dynamic System Prompt and the `genai.Content` history array, ensuring that instructions, structural maps, and recent human prompts are perfectly layered.