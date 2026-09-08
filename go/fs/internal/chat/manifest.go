package chat

import "uuid"

type DigestMeta struct {
	ID       uuid.UUID `json:"id"`
	Summary  string    `json:"summary"`
	IsSticky bool      `json:"is_sticky"`
}

type ThreadManifest struct {
	ThreadID    string `json:"thread_id"`
	ActiveShard string `json:"active_shard"`

	Digests map[uuid.UUID]DigestMeta `json:"digests"`
	Lenses  map[string][]uuid.UUID   `json:"lenses"`
}

func NewThreadManifest(threadID string) *ThreadManifest {
	return &ThreadManifest{
		ThreadID:    threadID,
		ActiveShard: "00000.jsonl",
		Digests:     make(map[uuid.UUID]DigestMeta),
		Lenses:      make(map[string][]uuid.UUID),
	}
}
