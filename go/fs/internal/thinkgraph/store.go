package thinkgraph

import (
	"context"
	"fmt"
)

// EventWriter is the shared contract for anything that persists chat state.
type EventWriter interface {
	AppendEvent(ctx context.Context, c *Chat, event Event) error
	SaveDigest(ctx context.Context, c *Chat, digest DigestMeta) error
}

// Ledger defines the physical file-system audit trail (Git source of truth).
type Ledger interface {
	EventWriter
}

// GraphRepository defines the fast KV index for O(1) non-linear traversal.
type GraphRepository interface {
	EventWriter

	// Lifecycle
	ClearChatGraph(ctx context.Context, c *Chat) error
	Close() error

	// Sync State
	GetSyncState(ctx context.Context, c *Chat) (*LedgerManifest, error)
	SetSyncState(ctx context.Context, c *Chat, manifest LedgerManifest) error

	// Read Paths (The Hot Path)
	GetTimeline(ctx context.Context, c *Chat) ([]Event, error)
	GetLenses(ctx context.Context, c *Chat, tag string) ([]Event, error)
	GetStickyDigests(ctx context.Context, c *Chat) ([]DigestMeta, error)
}

// Store unifies the File and DB brains, ensuring they stay in sync on writes.
type Store struct {
	graph  GraphRepository
	ledger Ledger
}

func NewStore(graph GraphRepository, ledger Ledger) *Store {
	return &Store{
		graph:  graph,
		ledger: ledger,
	}
}

// AppendEvent explicitly executes the dual-write to both Git and BadgerDB.
func (s *Store) AppendEvent(ctx context.Context, c *Chat, event Event) error {
	if err := s.ledger.AppendEvent(ctx, c, event); err != nil {
		return fmt.Errorf("ledger append failed: %w", err)
	}

	if err := s.graph.AppendEvent(ctx, c, event); err != nil {
		return fmt.Errorf("graph append failed: %w", err)
	}

	return nil
}

// SaveDigest executes the dual-write for long-term memory.
func (s *Store) SaveDigest(ctx context.Context, c *Chat, digest DigestMeta) error {
	if err := s.ledger.SaveDigest(ctx, c, digest); err != nil {
		return fmt.Errorf("ledger save digest failed: %w", err)
	}

	if err := s.graph.SaveDigest(ctx, c, digest); err != nil {
		return fmt.Errorf("graph save digest failed: %w", err)
	}

	return nil
}

// Graph returns the underlying GraphRepository for fast read-only queries.
func (s *Store) Graph() GraphRepository {
	return s.graph
}
