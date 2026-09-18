# ThinkSpace: Cognitive Architecture

## The Core Philosophy: Structured Cognitive States
ThinkSpace does not treat LLM context as a "delivery mechanism" or a bucket of strings to be dumped into an API. Instead, context is a **context creation mechanism**—a highly structured environment designed to orient the LLM, enable decision-making, and allow it to dig for details.

LLMs do not simply execute isolated tasks. They operate within a sequence of thought. Severing a prompt from its environment renders it meaningless (e.g., the prompt "Yes, do that" has no meaning without the active working memory preceding it). 

To enforce this, ThinkSpace orchestrates interactions using a **`ThinkBundle`**: a formalized representation of the AI's current cognitive state.

## The Four Pillars of a ThinkBundle
A `ThinkBundle` is composed of four distinct pillars. These pillars are divided into two categories: the **Material State** (what is fixed or explicitly placed on the desk) and the **Mutating State** (the evolving reality the LLM uses to navigate).

### 1. The Material State (Static & Explicit)
These pillars define the boundaries of reality and the tangible materials currently in hand.

*   **`SystemRules` (The Immutable Laws):** 
    Static and unchanging for the duration of a turn. This includes the Persona (e.g., "You are the Lead Architect"), the strict formatting constraints (e.g., "Output JSON only"), and the tool schemas available to the agent.
*   **`Workbench` (The Materialized Focus):** 
    Fairly static during a turn. This represents explicit physical files or specific data that have been loaded from disk and placed directly in front of the LLM. If the `Mapbook` is the map, the `Workbench` is the dirt in the agent's hands.

### 2. The Mutating State (Navigational & Temporal)
These pillars represent the shifting, active cognitive environment. They work in tandem: the Mapbook maps the available universe (both physical and conceptual), and the Trajectory is the immediate path moving through it.

*   **`Mapbook` (The Navigable Topology & Memory Index):** 
    The Mapbook maps both physical space and cognitive/temporal space. It is a set of layered domain guides that dynamically updates as the project evolves:
    *   *Structural Layer:* The physical layout of the repository.
    *   *Conceptual Layer:* Domain-specific constraints (e.g., "All Go code must reside in /src").
    *   *MacroTemporal Layer & Digest Routing:* As active conversational phases complete, they are compressed into historical Digests. The Mapbook updates to index these conceptual landmarks. As the user's focus shifts, the Mapbook serves as the routing mechanism that dictates *which* Digests are pulled back into the active Trajectory.
*   **`Trajectory` (The Active Cognitive Thread):** 
    This is the unbroken chain of state that brings the LLM to the current moment. Its contents shift based on the Mapbook's routing:
    *   *Long-Term Memory:* Digests and semantic tags dynamically routed into view by the Mapbook to provide background context without cluttering active thought.
    *   *Active Working Memory:* The cohesive, focused back-and-forth of the recent *N* turns. 
    *   *The Catalyst:* The immediate user prompt or sub-agent instruction. It is the "special" final event in the trajectory that triggers the evaluation of the surrounding working memory.

## Dual Context Phases
The `ThinkBundle` morphs depending on the actor's role in the lifecycle.

1.  **Orchestration Phase (The Manager):** 
    The Manager operates with a massive `Trajectory` (the focused working memory and routed Digests) and a complete `Mapbook`, but an empty `Workbench`. Its job is to use its cognitive thread to orient itself in the topology, decide what needs to be done, and delegate.
2.  **Execution Phase (The Sub-Agent):** 
    The Sub-Agent operates with a synthetic, highly-focused `Trajectory` (composed of the Manager's `context_digest` and specific instruction) and a populated `Workbench` (the specific files the Manager requested). 

## Observability & UI
Because a `ThinkBundle` is a structured cognitive state rather than a flat string, it is treated as a first-class data structure in the UI. 

ThinkSpace utilizes a full-screen **Context Inspector** overlay. This allows developers and users to view the exact `ThinkBundle` for multiple actors simultaneously (e.g., the Manager's pre-flight cognitive state vs. Agent 1's execution state). By separating the pillars visually, the system provides perfect observability into exactly *how* the AI was oriented before it made a decision.