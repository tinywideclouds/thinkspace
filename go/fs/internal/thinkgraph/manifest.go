package thinkgraph

import "uuid"

// DigestMeta represents a chronological timeline compression.
type DigestMeta struct {
	ID               uuid.UUID   `json:"id"`
	Summary          string      `json:"summary"`
	Promoted         bool        `json:"promoted"`
	Tags             []string    `json:"tags,omitempty"`
	SummarizedEvents []uuid.UUID `json:"summarized_events"`
	Tombstoned       bool        `json:"tombstoned"`
}

func (d DigestMeta) IsTombstoned() bool {
	return d.Tombstoned
}

// SpaceContext represents a persistent environmental rule or core memory.
type SpaceContext struct {
	ID         uuid.UUID `json:"id"`
	Content    string    `json:"content"`
	Tags       []string  `json:"tags,omitempty"`
	Tombstoned bool      `json:"tombstoned"`
}

func (sc SpaceContext) IsTombstoned() bool {
	return sc.Tombstoned
}

// TagDefinition provides the LLM with a definitive dictionary.
type TagDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// LensResult holds the unified slice of the semantic graph for a given tag.
type LensResult struct {
	Events        []Event
	Digests       []DigestMeta
	SpaceContexts []SpaceContext
}

// ChatManifest represents the cognitive graph state of a chat.
type ChatManifest struct {
	Digests       map[uuid.UUID]DigestMeta   `json:"digests"`
	SpaceContexts map[uuid.UUID]SpaceContext `json:"space_contexts"`
	Taxonomy      map[string]TagDefinition   `json:"taxonomy"`
	Lenses        map[string][]uuid.UUID     `json:"lenses"`
}

func NewChatManifest() *ChatManifest {
	return &ChatManifest{
		Digests:       make(map[uuid.UUID]DigestMeta),
		SpaceContexts: make(map[uuid.UUID]SpaceContext),
		Taxonomy:      make(map[string]TagDefinition),
		Lenses:        make(map[string][]uuid.UUID),
	}
}
