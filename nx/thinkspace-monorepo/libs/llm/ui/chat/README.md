# llm-ui-chat

This library provides the modern Angular (v22+) presentation components for the ThinkSpace chat interface. 

## Responsibilities
* Render the chat feeds, logs, and interactive CLI prompts.
* Provide strict "dumb" components utilizing modern Signal `input()` and `output()` APIs.
* Maintain strict separation of concerns with isolated HTML and SCSS files.

## Components
* `ChatFeedComponent`: Renders the linear message feed.
* `ChatInputComponent`: Standard user prompt entry.
* `ChatStrategyPromptComponent`: Interactive buttons for delegation strategy (Refine, Review, etc.).
* `ChatReviewPromptComponent`: Interactive approval for generated candidate branches.

## Rules
* **MUST NOT** hold business logic, connect to WebSockets, or inject State services directly.
* Data flows *in* via `[inputs]`, actions flow *out* via `(outputs)`.