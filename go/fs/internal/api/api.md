# API & Transport Layer

The `api` package provides the network boundary for the ThinkSpace application. It exposes the HTTP and WebSocket endpoints required to connect a web-based frontend to the internal orchestration and workspace engines.

## Core Components

* **JSON Protocol (`events.go`):** Defines a strict, strongly-typed envelope (`WSEvent`) and payload structures for all bi-directional WebSocket communication[cite: 39]. It maps domain actions (like streaming chat, delegation updates, and UI prompts) to discrete `EventType` constants (e.g., `chat_stream`, `agent_stream`, `request_review`)[cite: 39].
* **Web Server (`server.go`):** Initializes the routing multiplexer. 
  * The `/api/spaces` route allows the frontend to fetch the raw YAML configurations of all available domain plugins.
  * The `/ws` route handles the WebSocket upgrade, performs the initial handshake to emit available spaces, and enters the main inbound message loop[cite: 40].
* **WebSocket UI (`websocket.go`):** Implements the `session.UserInterface` for web clients. It safely marshals and writes outbound JSON events using a mutex (`writeMu`) to prevent concurrent write panics on the socket. It also handles blocking UI requests (like choosing a strategy or reviewing a branch) by leveraging Go channels (`strategyChan`, `reviewChan`) to wait for the client's inbound response[cite: 42].