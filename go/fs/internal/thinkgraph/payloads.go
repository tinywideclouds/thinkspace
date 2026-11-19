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
	IsSticky         bool        `json:"is_sticky"`
	SummarizedEvents []uuid.UUID `json:"summarized_events,omitempty"`
}

type BranchAnchor struct {
	ParentBranch string    `json:"parent_branch"`
	ForkPoint    uuid.UUID `json:"fork_point"`
}
