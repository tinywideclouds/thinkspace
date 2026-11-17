# LLM Integration Layer

The `llm` package serves as the bridge between the application's orchestration domain and generative AI models. It abstracts provider SDKs to enable deterministic testing, structured tool invocation, and response healing.

## Core Components

* **`ModelClient` Interface (`client.go`):** Defines the provider-agnostic streaming generation boundary (`GenerateContentStream` returning `iter.Seq2[StreamChunk, error]`). Tests substitute `MockModelClient` without hitting live APIs.
* **`ResponseHealer` Pipeline (`healer.go`):** Sanitizes malformed JSON outputs from models (`MarkdownBlockExtractor` $\rightarrow$ `JSONBoundaryExtractor` $\rightarrow$ `GoEscapeSanitizer`) before patch or digest deserialization.
* **`gemini.Adapter` (`gemini/adapter.go`):** Translates `cognition.ThinkBundle`, `tools.Registry`, and `spaces.ThinkStateConfig` into Google GenAI SDK requests (`compileSystemInstruction`, `compileHistory`, `geminitools.CompileCoreTools`) and normalizes streamed chunks, token usage, and function calls.