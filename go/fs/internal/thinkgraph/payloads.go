package thinkgraph

import "uuid"

// --- Concrete Domain Entities ---

type Prompt struct {
	Content string `json:"content"`
	Amends  string `json:"amends,omitempty"`
}

type ModelResponse struct {
	Content         string `json:"content"`
	PromptTokens    int    `json:"prompt_tokens,omitempty"`
	CandidateTokens int    `json:"candidate_tokens,omitempty"`
	TotalTokens     int    `json:"total_tokens,omitempty"`
}

type Digest struct {
	Summary          string      `json:"summary"`
	Promoted         bool        `json:"promoted"`
	SummarizedEvents []uuid.UUID `json:"summarized_events,omitempty"`
}
