package workspace

import "uuid"

type DigestMeta struct {
	ID       uuid.UUID `json:"id"`        // Matches the LedgerEvent ID in the .jsonl
	Summary  string    `json:"summary"`   // The compressed context
	IsSticky bool      `json:"is_sticky"` // Always include in context
}

type ThreadManifest struct {
	ThreadID    string `json:"thread_id"`
	ActiveShard string `json:"active_shard"` // e.g., "01000-01499.jsonl"

	// The active summaries available to the Context Assembler
	Digests map[uuid.UUID]DigestMeta `json:"digests"`

	// Lenses map a semantic tag to a list of relevant Event/Digest UUIDs
	Lenses map[string][]uuid.UUID `json:"lenses"`
}

func NewThreadManifest(threadID string) *ThreadManifest {
	return &ThreadManifest{
		ThreadID:    threadID,
		ActiveShard: "00000.jsonl",
		Digests:     make(map[uuid.UUID]DigestMeta),
		Lenses:      make(map[string][]uuid.UUID),
	}
}
