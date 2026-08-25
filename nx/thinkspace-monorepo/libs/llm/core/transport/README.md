# llm-core-transport

This library provides the low-level WebSocket connection management for the ThinkSpace application.

## Responsibilities
* Manage the RxJS `WebSocketSubject` lifecycle (connect, disconnect, retry).
* Handle the raw JSON stringification and parsing of the `WSEvent` Protobuf envelope using `@bufbuild/protobuf` (via `toJsonString` and `fromJson`).
* Safely emit raw Protobuf messages to the upper layers.

## Rules
* **DO NOT** import domain logic or state management here.
* This library operates strictly on Protobuf types. It does not know what the application does with the data.