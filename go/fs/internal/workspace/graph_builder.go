package workspace

import (
	"context"
	"encoding/json"

	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

// WorkspaceGraphBuilder inflates events into materialized graph nodes.
// It intercepts Candidate Proposals to hydrate the Node's MaterializedState with Git diffs.
type WorkspaceGraphBuilder struct {
	engine ChatVersioner
}

func NewWorkspaceGraphBuilder(engine ChatVersioner) *WorkspaceGraphBuilder {
	return &WorkspaceGraphBuilder{
		engine: engine,
	}
}

func (b *WorkspaceGraphBuilder) Build(ctx context.Context, c *thinkgraph.ChatAddress, ev thinkgraph.Event) (thinkgraph.Node, error) {
	node := thinkgraph.Node{
		Event: ev,
	}

	if ev.Type == EventCandidateProposal {
		var payload CandidateProposal
		if err := json.Unmarshal(ev.Data, &payload); err == nil {
			// Hydrate the Git diff from the physical ChatEngine
			diff, err := b.engine.ReadCandidateDiff(ctx, c.ChatID, payload.CandidateID)
			if err == nil && diff != "" {
				// Cache the diff natively on the graph read-model
				stateBytes, _ := json.Marshal(map[string]string{"diff": diff})
				node.MaterializedState = stateBytes
			}
		}
	}

	return node, nil
}
