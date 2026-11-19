# Application Entry Points

The system provides two distinct binaries to interact with the ThinkSpace orchestration layer: a command-line interface and a WebSocket-based API server[cite: 6]. Both entry points share the same underlying domain logic, state engines, and LLM orchestration, but inject different user interfaces[cite: 6].

## CLI Application (`main.go`)

The CLI binary provides a synchronous, terminal-based loop for interacting with the workspace[cite: 6].

### Configuration Flags
* `-chat`: The name of the exploration chat (default: `"unit-circle-test"`)[cite: 6].
* `-engine`: State engine backend to use, either `'gogit'` or `'exec'` (default: `"gogit"`)[cite: 6].
* `-space`: The ThinkSpace directory to use (default: `"sandbox"`)[cite: 6].
* `-thinkstate`: The ThinkState configuration to load (default: `"intake"`)[cite: 6].
* `-debug-llm`: DEV SHORTCUT flag to record the full compiled `ThinkBundle` context to temp Markdown files (`<os.TempDir()>/thinkspace-llm/`) and log the file path on every LLM call (default: `false`).

### Execution Flow
1. **Initialization:** Loads environment variables and configures the GenAI client (wrapping it in `llm.LoggingModelClient` if `-debug-llm` is set)[cite: 6].
2. **Workspace Setup:** Provisions the workspace directory and loads the requested domain configuration from the registry[cite: 6].
3. **Dependency Injection:** Wires the `ServiceManager`, `Adapter`, `FanOutFlow`, and injects the `TerminalUI` into the supervisor[cite: 6].
4. **Session Resumption:** Checks if the chat directory exists[cite: 6]. If it does not, it creates a new chat and injects an initial prompt[cite: 6]. If it does, it loads the read-model from the BadgerDB graph, utilizing the `Rehydrator` to fast-forward any missing events from the append-only ledger (`.jsonl`)[cite: 6]. It then blocks for user terminal input[cite: 6].
5. **Execution:** Triggers a single `RunTurn` via the `Manager`, blocking until the LLM generation, tool execution, and user review phases are complete[cite: 6].

---

## Web Server (`server_main.go`)

The web server binary hosts the HTTP and WebSocket endpoints required to attach a browser-based frontend[cite: 6].

### Configuration Flags
* `-port`: The port for the API server (default: `8080`)[cite: 6].
* `-root`: Root directory for the ThinkSpace repositories (default: `"thinkspace-root"`)[cite: 6].
* `-engine`: State engine backend to use, either `'gogit'` or `'exec'` (default: `"gogit"`)[cite: 6].
* `-use-skeleton`: DEV SHORTCUT flag to seed local directories using the internal skeleton configuration (default: `false`)[cite: 6].
* `-clean`: DEV SHORTCUT flag to wipe the local BadgerDB graph index on startup for a fresh rebuild from the ledger (default: `false`)[cite: 6].
* `-debug-llm`: DEV SHORTCUT flag to record the full compiled `ThinkBundle` context to temp Markdown files (`<os.TempDir()>/thinkspace-llm/`) and log the file path on every LLM call (default: `false`).

### Execution Flow
1. **Initialization:** Loads environment variables and configures the GenAI client (wrapping it in `llm.LoggingModelClient` if `-debug-llm` is set)[cite: 6].
2. **Workspace Setup:** Initializes the target base root directory and sets up the selected Git state engine factory via the `ServiceManager`[cite: 6].
3. **Registry Loading:** Loads all available domain configurations from the `configs` directory to serve them via the `/api/spaces` and `/api/config` routes[cite: 6].
4. **Server Initialization:** Injects the shared domain dependencies into the API and WebSocket server components[cite: 6].
5. **Listen and Serve:** Starts listening on the specified port for inbound HTTP and WebSocket connections[cite: 6].