# Tools Architecture

The `tools` package defines the strict, domain-centric capabilities available to the LLM during a session turn. 

This architecture strictly separates **execution logic** from **provider schema generation** (e.g., Gemini's `genai.Tool`). This ensures the core orchestration layer remains completely agnostic to the underlying AI model.

## Core Concepts

### 1. The `Tool` Interface
Every tool in ThinkSpace must implement the minimal `Tool` interface:

```go
type Tool interface {
    Name() ToolName
    DefaultDescription() string
    Execute(ctx context.Context, executionContext ExecutionContext) (bool, error)
}
```
Notice what is *missing*: there is no schema definition, no parameter mapping, and no LLM SDK imports. The tool's only job is to know its name, provide a fallback description, and execute its side effects (like querying the graph or dispatching sub-agents) using strictly typed Go structs.

### 2. The Tool Registry
Tools are instantiated and registered into a central `Registry` when a `ChatSession` boots up or transitions. The `TurnCoordinator` uses this registry for O(1) execution routing. When the LLM requests a tool call, the coordinator simply looks up the tool by name and invokes `Execute()`.

## ThinkState Integration

ThinkSpace is designed around specialized operational modes called **ThinkStates** (e.g., `golang`, `concept`, `intake`). The active `ThinkState` directly influences how tools are presented to the LLM.

When the LLM Client binds the tools (`ModelClient.BindTools(registry, config)`), it passes in the active `ThinkStateConfig`. The provider-specific compilers (e.g., `internal/session/tools/gemini`) use this configuration to dynamically override tool behaviors:

1. **Description Overrides:** A tool like `propose_change` might need a different description when operating in a creative conceptual space versus a strict Golang engineering space. The `ThinkStateConfig` allows the YAML definition to override the `DefaultDescription`.
2. **Parameter Shaping:** The configuration can override the descriptions of specific parameters (e.g., enforcing that `target_files` should only contain `.md` files in a documentation state).

This allows a single, generic domain tool (`propose_change`) to be heavily specialized and contextualized purely through YAML configuration, without changing the Go code.

## Adding a New LLM Provider

Because of this architectural boundary, adding a new LLM provider (like OpenAI or Anthropic) requires **zero changes** to the core orchestration or the tools themselves.

To add a new provider:
1. Implement the `llm.ModelClient` interface in a new adapter package (e.g., `internal/llm/openai/adapter.go`).
2. Implement `BindTools(registry, config)` on your adapter. Extract the concrete tools from the `registry` and pass them to provider-specific compiler functions.
3. Write provider-specific compilers (e.g., `internal/session/tools/openai/compiler.go`) that manually map the strict Go request structs (like `ProposeChangeRequest`) into the specific JSON schema format required by that provider's API. Cache this compiled state inside the adapter.