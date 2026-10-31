package thinkgraph

import "context"

// GraphBuilder inflates flat ledger events into rich, stateful Graph Nodes.
// It is responsible for fetching external data and caching it in the Node's MaterializedState.
type GraphBuilder interface {
	Build(ctx context.Context, c *Chat, ev Event) (Node, error)
}

// DefaultBuilder acts as a passthrough baseline if no MaterializedState inflation is needed.
type DefaultBuilder struct{}

func (d *DefaultBuilder) Build(ctx context.Context, c *Chat, ev Event) (Node, error) {
	return Node{
		Event: ev,
	}, nil
}
