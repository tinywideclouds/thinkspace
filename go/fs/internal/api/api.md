# API Architecture

The `api` package provides the network transport layer for ThinkSpace. It acts as a strict boundary, separating static state management from real-time agent orchestration, and adapting network protocols into internal domain interfaces.

## REST Management API (`management.go`)
Provides stateless, traditional HTTP endpoints for the frontend to synchronize the UI state.
*   `GET /api/config`: Retrieves startup configuration defaults.
*   `GET /api/spaces`: Lists all available ThinkSpaces.
*   `GET /api/spaces/{space_id}/chats`: Lists all chats for a specific space.
*   `POST /api/spaces/{space_id}/chats`: Creates a new chat directory and its underlying `meta.json`.

These endpoints interface exclusively with the `workspace.ServiceManager` to perform disk reads/writes without triggering any LLM connections or Git state mechanics.

## WebSocket Server (`server.go`)
Handles the long-lived, real-time agentic loop. 
*   **Connection:** Clients connect via `/ws`.
*   **Payload Routing:** The master envelope (`WSEvent`) uses strongly-typed payloads defined by the Protobuf contract (`events.proto`). 
*   **Dynamic Resolution:** When a `SubmitPromptPayload` is received, it extracts both `space_id` and `chat_id`. The WebSocket server asks the `ServiceManager` for the warmed-up `workspace.Service` instance tied to that specific space, starts the thread, and hands execution over to the `session.Coordinator`.

## The Facade Pattern (`facade.go`)
The application domain never directly touches Protobuf generated structs. The `EventFacade` translates inbound Protobuf bytes into pure Go domain structs (`InboundEvent`), and translates outbound domain actions into Protobuf bytes. This ensures the transport protocol can evolve without corrupting the core orchestration logic.

## UI Adapters
To decouple the orchestration logic (`session.Coordinator`, `flows`) from the network transport, the API layer provides adapter implementations:
*   **`WebSocketUI`:** Implements the `session.UI` interface. It buffers and streams text chunks back to the client and blocks on channels to wait for human input (like strategy selection or candidate review).
*   **`WebSocketEmitter`:** Implements the `flows.FlowEmitter` interface, translating complex sub-agent lifecycle events (spawn, status, trace, complete) into Protobuf `FlowEventPayload` messages for real-time frontend observation.