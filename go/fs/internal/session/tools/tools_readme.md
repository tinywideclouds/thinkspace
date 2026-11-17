# Tools Architecture

The `tools` package defines the strict, domain-centric capabilities available to the LLM during a session turn.

This architecture strictly separates **execution logic** from **provider schema generation** (e.g., `internal/session/tools/gemini`). This ensures the core orchestration layer remains agnostic to the underlying AI model.

## Core Concepts

### 1. The `Tool` Interface
Every tool in ThinkSpace implements the `Tool` interface:

```go
type Tool interface {
    Name() ToolName
    DefaultDescription() string
    Execute(ctx context.Context, executionContext ExecutionContext) (ToolResponse, error)
}