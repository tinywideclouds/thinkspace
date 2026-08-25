# Application Entry Points

The system provides two distinct binaries to interact with the ThinkSpace orchestration layer: a command-line interface and a WebSocket-based API server. Both entry points share the same underlying domain logic, state engines, and LLM manager, but inject different user interfaces into the orchestrator.

## CLI Application (`main.go`)

The CLI binary provides a synchronous, terminal-based loop for interacting with the workspace[cite: 1]. 

### Configuration Flags
*   `-chat`: The name of the exploration thread (default: "unit-circle-test")[cite: 1].
*   `-engine`: State engine backend to use, either 'gogit' or 'exec' (default: "gogit")[cite: 1].
*   `-space`: The ThinkSpace directory to use (default: "sandbox")[cite: 1].
*   `-domain`: The domain configuration YAML to load (default: "golang")[cite: 1].

### Execution Flow
1. **Initialization:** Loads environment variables and configures the GenAI client[cite: 1].
2. **Workspace Setup:** Provisions the workspace directory and loads the requested domain configuration from the registry[cite: 1].
3. **Dependency Injection:** Wires the `Service`, `Manager`, `FanOutFlow`, and injects the `TerminalUI` into the `Coordinator`[cite: 1].
4. **Session Resumption:** Checks if the thread directory exists. If it does not, it creates a new thread and injects an initial prompt[cite: 1]. If it does, it parses the append-only ledger (`conversation.jsonl`), rebuilds the history, and blocks for user terminal input[cite: 1].
5. **Execution:** Triggers a single `ExecuteTurn` via the `Coordinator`, blocking until the LLM generation, tool execution, and user review phases are complete[cite: 1].

---

## Web Server (`server_main.go`)

The web server binary hosts the HTTP and WebSocket endpoints required to attach a browser-based frontend[cite: 2]. 

### Configuration Flags
*   `-port`: The port for the API server (default: 8080)[cite: 2].
*   `-space`: The ThinkSpace directory to use (default: "sandbox")[cite: 2].
*   `-engine`: State engine backend to use, either 'gogit' or 'exec' (default: "gogit")[cite: 2].

### Execution Flow
1. **Initialization:** Loads environment variables and configures the GenAI client[cite: 2].
2. **Workspace Setup:** Initializes the target workspace directory and sets up the selected Git state engine[cite: 2].
3. **Registry Loading:** Loads all available domain configurations from the `configs` directory to serve them via the `/api/spaces` route[cite: 2].
4. **Server Initialization:** Injects the shared domain dependencies into `api.NewServer`[cite: 2].
5. **Listen and Serve:** Starts listening on the specified port for inbound HTTP and WebSocket connections[cite: 2].