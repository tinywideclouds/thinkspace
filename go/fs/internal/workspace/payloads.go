package workspace

import "github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"

const (
	EventCandidateResolution thinkgraph.EventType = "candidate_resolution"
	EventFileUpdate          thinkgraph.EventType = "file_update"
)

type CandidateResolution struct {
	CandidateID string `json:"candidate_id"`
	Accepted    bool   `json:"accepted"`
	Reason      string `json:"reason"`
}

type FileUpdate struct {
	Path   string `json:"path"`
	Action string `json:"action"` // e.g., "create", "modify", "delete"
}
