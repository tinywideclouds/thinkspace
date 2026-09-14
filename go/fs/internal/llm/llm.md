# LLM Integration Layer

The `llm` package serves as the bridge between the application's orchestration domain and the generative AI models. It abstracts the underlying SDKs to enable deterministic testing and structured tool invocation.

## Core Components

* **`ModelClient` Interface:** Defines the boundary for streaming generation. By wrapping the Google GenAI SDK (`GenAIClient`), the system can easily substitute a `MockModelClient` during testing to yield predefined responses without hitting real APIs or incurring costs.
* **`Adapter`:** The central LLM interface handler. It bridges the pure workspace domain events into the Google GenAI payload formats. It is responsible for:
  * **History Translation (`BuildHistory`):** Converting the application's custom workspace events (prompts, raw tool actions, and resolutions) into the strictly formatted `genai.Content` schema expected by the model.
  * **Stream Generation:** Initiating the generation streams with system prompts, domain tools, and temperature configuration.
  * **Tool Interception:** Parsing streaming response chunks to detect and extract `ToolCall` requests generically.
* **`SubAgentExecutor` (`subagent.go`):** A factory that creates specialized, isolated execution closures for sub-agents.
  * It forces the model to respond in `application/json` format.
  * It multiplexes the streaming text back to a UI token channel.
  * It automatically parses the JSON response into physical files within a target sandbox directory.
  * It maintains a localized shadow ledger (`trace.jsonl`) of the sub-agent's prompt and exact generation.

## Tool Declarations

Tools and their precise schemas (e.g., `propose_change`, `query_lens`) are defined in the specific `spaces` domain implementations (like `internal/spaces/golang/space.go`), relying on the `genai.Schema` primitives exposed through the `ThinkSpace` interface.