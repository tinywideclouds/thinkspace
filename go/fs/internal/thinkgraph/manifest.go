package thinkgraph

import "uuid"

// DigestMeta represents crystallized long-term memory.
// IsSticky combined with Tags defines a Conditional Law. If Tags is empty, it is globally sticky.
type DigestMeta struct {
	ID               uuid.UUID   `json:"id"`
	Summary          string      `json:"summary"`
	IsSticky         bool        `json:"is_sticky"`
	Tags             []string    `json:"tags,omitempty"`
	SummarizedEvents []uuid.UUID `json:"summarized_events"` // Non-linear zoom linkage
}

type ChatManifest struct {
	ChatID      string `json:"chat_id"`
	ActiveShard string `json:"active_shard"`

	Digests map[uuid.UUID]DigestMeta `json:"digests"`
	Lenses  map[string][]uuid.UUID   `json:"lenses"`
}

func NewChatManifest(chatID string) *ChatManifest {
	return &ChatManifest{
		ChatID:      chatID,
		ActiveShard: "00000.jsonl",
		Digests:     make(map[uuid.UUID]DigestMeta),
		Lenses:      make(map[string][]uuid.UUID),
	}
}
