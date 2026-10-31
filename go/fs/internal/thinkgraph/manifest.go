package thinkgraph

import "uuid"

type DigestMeta struct {
	ID               uuid.UUID   `json:"id"`
	Summary          string      `json:"summary"`
	Tags             []string    `json:"tags,omitempty"`
	Promoted         bool        `json:"promoted"`
	SummarizedEvents []uuid.UUID `json:"summarized_events,omitempty"`
	Tombstoned       bool        `json:"tombstoned"`
}

type SpaceContext struct {
	ID         uuid.UUID `json:"id"`
	Content    string    `json:"content"`
	Tags       []string  `json:"tags,omitempty"`
	Tombstoned bool      `json:"tombstoned"`
}

type ChatManifest struct {
	Digests       map[uuid.UUID]DigestMeta
	SpaceContexts map[uuid.UUID]SpaceContext
}

func NewChatManifest() *ChatManifest {
	return &ChatManifest{
		Digests:       make(map[uuid.UUID]DigestMeta),
		SpaceContexts: make(map[uuid.UUID]SpaceContext),
	}
}

// LensResult represents a targeted vertical slice through the read model.
type LensResult struct {
	Nodes         []Node
	Digests       []DigestMeta
	SpaceContexts []SpaceContext
}
