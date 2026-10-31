package workspace

import "github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"

const (
	EventCandidateProposal   thinkgraph.EventType = "candidate_proposal"
	EventCandidateResolution thinkgraph.EventType = "candidate_resolution"
	EventFileUpdate          thinkgraph.EventType = "file_update"
)

type CandidateProposalPayload struct {
	CandidateID string   `json:"candidate_id"`
	Description string   `json:"description"`
	Files       []string `json:"files"`
}

type CandidateResolutionPayload struct {
	CandidateID string `json:"candidate_id"`
	Accepted    bool   `json:"accepted"`
	Reason      string `json:"reason"`
}

type FileUpdatePayload struct {
	Path   string `json:"path"`
	Action string `json:"action"` // e.g., "create", "modify", "delete"
}
