
* **Browser Logging:** Backend `slog` events can be broadcast over a dedicated WebSocket channel directly to a toggleable terminal component in the UI. This provides immediate developer visibility into the Go service without polluting the semantic graph or requiring window switching.
* **UI-Telemetry:** High-frequency transient updates (e.g., "Agent 1 compiling", "Agent 2 testing") ride a lightweight, ephemeral telemetry bus. The Angular store routes these directly to the active Flow card to animate progress spinners or status badges. Because they bypass `thinkgraph` and BadgerDB entirely, they create zero I/O drag and vanish on a page refresh, leaving only the durable milestones.

This creates three distinct data planes: the Ledger/Graph for structural truth, the log stream for raw diagnostics, and the telemetry stream for real-time UI animation.
