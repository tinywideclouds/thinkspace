# LLM Integration Layer

The `llm` package serves as the bridge between the application's orchestration domain and the generative AI models. It abstracts the underlying SDKs to enable deterministic testing and structured tool invocation[cite: 34, 35].

## Core Components

* **`ModelClient` Interface:** Defines the boundary for streaming generation[cite: 34]. By wrapping the Google GenAI SDK (`GenAIClient`), the system can easily substitute a `MockModelClient` during testing to yield predefined responses without hitting real APIs or incurring costs[cite: 34, 36].
* **`Manager`:** The central LLM session handler. It is responsible for:
  * **History Translation (`BuildHistory`):** Converting the application's custom workspace events (prompts, tool calls, resolutions) into the strictly formatted `genai.Content` schema expected by the model.
  * **Stream Generation:** Configuring and initiating the generation streams with system prompts, tools, and temperature settings.
  * **Tool Interception:** Parsing streaming chunks to detect and extract `ToolCall` requests generically[cite: 35].
* **`SubAgentExecutor` (`subagent.go`):** A factory that creates specialized, isolated execution closures for sub-agents. 
  * It forces the model to respond in `application/json` format.
  * It multiplexes the streaming text back to a UI token channel[cite: 37].
  * It automatically parses the JSON response into physical files within a target sandbox directory[cite: 37].
  * It maintains a localized shadow ledger (`trace.jsonl`) of the sub-agent's prompt and exact generation[cite: 37].

## Tool Declarations

The package also centralizes the schema definitions for AI tools, such as `propose_change`, ensuring the LLM knows exactly how to format its requests when interacting with the workspace state[cite: 38].