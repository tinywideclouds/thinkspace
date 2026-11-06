package thinkgraph

import (
	"context"
	"fmt"
	"log/slog"
)

// Dispatcher routes write operations to both the append-only Ledger and the stateful GraphStore.
type Dispatcher struct {
	graphStore *GraphStore
	ledger     EventWriter
}

func NewDispatcher(graphStore *GraphStore, ledger EventWriter) *Dispatcher {
	if graphStore == nil {
		slog.Warn("Dispatcher initialized without a GraphStore sink. Graph projections will not be updated.")
	}
	if ledger == nil {
		slog.Warn("Dispatcher initialized without a Ledger sink. Persistent event log will not be updated.")
	}

	return &Dispatcher{
		graphStore: graphStore,
		ledger:     ledger,
	}
}

func (d *Dispatcher) AppendEvent(ctx context.Context, c *ChatAddress, event Event) error {
	// Lock in the mathematical DAG identity before fanning out to sinks
	if event.GraphID.IsZero() {
		event.GraphID = c.GraphID
	}

	if d.ledger != nil {
		if err := d.ledger.AppendEvent(ctx, c, event); err != nil {
			return fmt.Errorf("ledger append failed: %w", err)
		}
	}
	if d.graphStore != nil {
		if err := d.graphStore.AppendEvent(ctx, c, event); err != nil {
			return fmt.Errorf("graph store append failed: %w", err)
		}
	}
	return nil
}

func (d *Dispatcher) SaveDigest(ctx context.Context, c *ChatAddress, digest DigestMeta) error {
	if d.ledger != nil {
		if err := d.ledger.SaveDigest(ctx, c, digest); err != nil {
			return fmt.Errorf("ledger save digest failed: %w", err)
		}
	}
	if d.graphStore != nil {
		if err := d.graphStore.SaveDigest(ctx, c, digest); err != nil {
			return fmt.Errorf("graph store save digest failed: %w", err)
		}
	}
	return nil
}

func (d *Dispatcher) SaveSpaceContext(ctx context.Context, c *ChatAddress, spaceCtx SpaceContext) error {
	if d.ledger != nil {
		if err := d.ledger.SaveSpaceContext(ctx, c, spaceCtx); err != nil {
			return fmt.Errorf("ledger save space context failed: %w", err)
		}
	}
	if d.graphStore != nil {
		if err := d.graphStore.SaveSpaceContext(ctx, c, spaceCtx); err != nil {
			return fmt.Errorf("graph store save space context failed: %w", err)
		}
	}
	return nil
}

func (d *Dispatcher) SaveMutation(ctx context.Context, c *ChatAddress, mutation MutationEvent) error {
	if d.ledger != nil {
		if err := d.ledger.SaveMutation(ctx, c, mutation); err != nil {
			return fmt.Errorf("ledger save mutation failed: %w", err)
		}
	}
	if d.graphStore != nil {
		if err := d.graphStore.SaveMutation(ctx, c, mutation); err != nil {
			return fmt.Errorf("graph store save mutation failed: %w", err)
		}
	}
	return nil
}

func (d *Dispatcher) GraphStore() *GraphStore {
	return d.graphStore
}
