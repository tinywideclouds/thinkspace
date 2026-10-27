package thinkgraph

import (
	"context"
	"fmt"
)

// EventWriter is the shared contract for anything that persists chat state.
type EventWriter interface {
	AppendEvent(ctx context.Context, c *Chat, event Event) error
	SaveDigest(ctx context.Context, c *Chat, digest DigestMeta) error
	SaveSpaceContext(ctx context.Context, c *Chat, spaceCtx SpaceContext) error
	SaveMutation(ctx context.Context, c *Chat, mutation MutationEvent) error
}

// Ledger defines the physical file-system audit trail.
type Ledger interface {
	EventWriter
}

// GraphRepository defines the fast KV index for non-linear traversal.
type GraphRepository interface {
	EventWriter

	ClearChatGraph(ctx context.Context, c *Chat) error
	Close() error

	GetSyncState(ctx context.Context, c *Chat) (*LedgerManifest, error)
	SetSyncState(ctx context.Context, c *Chat, manifest LedgerManifest) error

	// Single-branch reads
	GetBranchTimeline(ctx context.Context, c *Chat, branchID string) ([]Event, error)
	GetBranchTimelinePage(ctx context.Context, c *Chat, branchID string, cursor string, limit int) ([]Event, string, bool, error)
	WalkBranchReverse(ctx context.Context, c *Chat, branchID string, startCursor string, fn func(Event) bool) error

	GetLenses(ctx context.Context, c *Chat, tag string) (LensResult, error)
	GetStickyDigests(ctx context.Context, c *Chat) ([]DigestMeta, error)
	GetDigests(ctx context.Context, c *Chat) ([]DigestMeta, error)
	GetSpaceContexts(ctx context.Context, c *Chat) ([]SpaceContext, error)
	GetEvents(ctx context.Context, c *Chat, ids []string) ([]Event, error)
}

// Store unifies the File and DB brains.
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

func (s *Store) AppendEvent(ctx context.Context, c *Chat, event Event) error {
	if err := s.ledger.AppendEvent(ctx, c, event); err != nil {
		return fmt.Errorf("ledger append failed: %w", err)
	}
	if err := s.graph.AppendEvent(ctx, c, event); err != nil {
		return fmt.Errorf("graph append failed: %w", err)
	}
	return nil
}

func (s *Store) SaveDigest(ctx context.Context, c *Chat, digest DigestMeta) error {
	if err := s.ledger.SaveDigest(ctx, c, digest); err != nil {
		return fmt.Errorf("ledger save digest failed: %w", err)
	}
	if err := s.graph.SaveDigest(ctx, c, digest); err != nil {
		return fmt.Errorf("graph save digest failed: %w", err)
	}
	return nil
}

func (s *Store) SaveSpaceContext(ctx context.Context, c *Chat, spaceCtx SpaceContext) error {
	if err := s.ledger.SaveSpaceContext(ctx, c, spaceCtx); err != nil {
		return fmt.Errorf("ledger save space context failed: %w", err)
	}
	if err := s.graph.SaveSpaceContext(ctx, c, spaceCtx); err != nil {
		return fmt.Errorf("graph save space context failed: %w", err)
	}
	return nil
}

func (s *Store) SaveMutation(ctx context.Context, c *Chat, mutation MutationEvent) error {
	if err := s.ledger.SaveMutation(ctx, c, mutation); err != nil {
		return fmt.Errorf("ledger save mutation failed: %w", err)
	}
	if err := s.graph.SaveMutation(ctx, c, mutation); err != nil {
		return fmt.Errorf("graph save mutation failed: %w", err)
	}
	return nil
}

func (s *Store) Graph() GraphRepository {
	return s.graph
}
