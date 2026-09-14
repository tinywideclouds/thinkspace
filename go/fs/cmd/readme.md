# Application Entry Points

The system provides two distinct binaries to interact with the ThinkSpace orchestration layer: a command-line interface and a WebSocket-based API server. Both entry points share the same underlying domain logic, state engines, and LLM orchestration, but inject different user interfaces.

## CLI Application (`main.go`)

The CLI binary provides a synchronous, terminal-based loop for interacting with the workspace.

### Configuration Flags
*   `-chat`: The name of the exploration thread (default: "unit-circle-test").
*   `-engine`: State engine backend to use, either 'gogit' or 'exec' (default: "gogit").
*   `-space`: The ThinkSpace directory to use (default: "sandbox").
*   `-domain`: The domain configuration YAML to load (default: "golang").

### Execution Flow
1. **Initialization:** Loads environment variables and configures the GenAI client.
2. **Workspace Setup:** Provisions the workspace directory and loads the requested domain configuration from the registry.
3. **Dependency Injection:** Wires the `Service`, `Adapter`, `FanOutFlow`, and injects the `TerminalUI` into the `Coordinator`.
4. **Session Resumption:** Checks if the thread directory exists. If it does not, it creates a new thread and injects an initial prompt. If it does, it parses the append-only ledger (`conversation.jsonl`), rebuilds the history, and blocks for user terminal input.
5. **Execution:** Triggers a single `ExecuteTurn` via the `Coordinator`, blocking until the LLM generation, tool execution, and user review phases are complete.

---

## Web Server (`server_main.go`)

The web server binary hosts the HTTP and WebSocket endpoints required to attach a browser-based frontend.

### Configuration Flags
*   `-port`: The port for the API server (default: 8080).
*   `-root`: Root directory for the ThinkSpace repositories (default: "thinkspace-root").
*   `-engine`: State engine backend to use, either 'gogit' or 'exec' (default: "gogit").
*   `-use-skeleton`: DEV SHORTCUT flag to seed local directories using the internal skeleton configuration (default: false).

### Execution Flow
1. **Initialization:** Loads environment variables and configures the GenAI client.
2. **Workspace Setup:** Initializes the target base root directory and sets up the selected Git state engine factory via the `ServiceManager`.
3. **Registry Loading:** Loads all available domain configurations from the `configs` directory to serve them via the `/api/spaces` route.
4. **Server Initialization:** Injects the shared domain dependencies into the API and WebSocket server components.
5. **Listen and Serve:** Starts listening on the specified port for inbound HTTP and WebSocket connections.