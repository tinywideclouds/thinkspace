package telemetry

// TokenUsage tracks the LLM token budget for a specific generation request.
type TokenUsage struct {
	PromptTokens    int `json:"prompt_tokens"`
	CandidateTokens int `json:"candidate_tokens"`
	TotalTokens     int `json:"total_tokens"`
}
