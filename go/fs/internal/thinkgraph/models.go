package thinkgraph

// Chat represents the temporal timeline of a conversation and its physical boundaries.
type Chat struct {
	ID      string
	SpaceID string
}

// GraphData represents the backend's reconstructed memory state loaded from disk.
type GraphData struct {
	Digests  []DigestMeta
	Timeline []Event
}
