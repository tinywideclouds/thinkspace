package chat

// Thread represents the temporal timeline of a conversation.
type Thread struct {
	ID         string
	SpaceID    string
	Branch     string // e.g., "chat/quantum-sim"
	LedgerPath string // Absolute path to conversation.jsonl
	Dir        string // Absolute path to the thread's working directory
}

// ChatGraph represents the structured memory graph loaded from the ledger.
// This sets up the Playback Engine to return navigable context rather than a flat array.
type ChatGraph struct {
	Digests      []DigestMeta
	RecentEvents []Event
	Map          []any
}
