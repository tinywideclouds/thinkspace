# llm-core-protos

This library contains the generated TypeScript bindings for the ThinkSpace Protobuf definitions. It serves as the strict network boundary contract between the Golang orchestration backend and the Angular frontend.

## Responsibilities
* House the generated `@bufbuild/protobuf` schemas (`events.proto`).
* Provide strongly-typed `WSEvent` wrappers and payload structures.
* Ensure type safety for JSON serialization/deserialization over WebSockets.

## Rules
* **DO NOT** write manual business logic in this library.
* **DO NOT** modify the generated `.ts` files manually. Always use the `buf generate` pipeline to update the schemas.