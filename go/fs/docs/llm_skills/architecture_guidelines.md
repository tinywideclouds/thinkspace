# LLM Skill: ThinkSpace Architecture & "SaaS Brain" Guardrails

When working on this codebase, **DO NOT apply cloud SaaS, multi-tenant, or highly-concurrent enterprise patterns**.

This application is a **local-first, single-user workspace tool** (similar to a local Jupyter server or a desktop Electron app backed by Go).

### 1. The Single Session Principle
*   **There is only one user.** They interact with exactly one `ChatSession` at a time.
*   The `Server` struct holds a single `activeSession *session.ChatSession`. 
*   We do not use maps of sessions. We do not use Redis. We do not use JWTs. 

### 2. Stateful Domain Objects Over Stateless REST
*   The `ChatSession` is a stateful, long-lived domain object. It is hydrated once from disk (`meta.json`) via `session.OpenChat()` when the user switches to a conversation.
*   Do **not** re-read fundamental setup files (like `meta.json`) on every single websocket message. 
*   WebSockets are just a dumb transport pipe. The application logic lives in standard Go methods (like `session.RunTurn()`), which natively use the state already loaded into the object.

### 3. Concrete Structs Over Abstract Detachment
*   Avoid detached "Managers" that pass string IDs down 10 layers of function calls. If an action pertains to a chat, call a method on the `ChatSession` object itself.
*   The cognitive graph state (Digests, Lenses, Contexts) is managed via the explicit `thinkgraph.ChatManifest` struct, avoiding raw unstructured map/slice passing.

**TL;DR:** Write simple, object-oriented Go code designed for a single user on a local machine. Keep state in memory as long as the user is looking at it, and let garbage collection handle the rest.