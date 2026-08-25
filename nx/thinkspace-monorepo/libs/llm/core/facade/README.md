# llm-core-facade

This library acts as an Anti-Corruption Layer (ACL) / Facade between the network transport (Protobuf) and the frontend application domain.

## Responsibilities
* Define the core TypeScript Domain Interfaces (`DomainEvent`, `DomainDelegationStrategy`, etc.).
* Translate incoming Protobuf `WSEvent` schemas into clean, decoupled Domain types.
* Translate outgoing Domain actions into Protobuf `WSEvent` objects ready for the transport layer.

## Rules
* **MUST** consist of pure, stateless functions.
* **MUST** be the *only* library in the workspace (outside of transport) that imports from `@org/llm-core-protos`. 
* The UI and State layers must never know Protobuf exists.