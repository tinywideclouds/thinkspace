package thinkgraph

import (
	"context"
	"fmt"
)

// GraphStore wraps the raw BadgerDB GraphRepository and the GraphBuilder.
// It implements the EventWriter interface by projecting flat Events into rich Nodes on the fly.
type GraphStore struct {
	repo    GraphRepository
	builder GraphBuilder
}

func NewGraphStore(repo GraphRepository, builder GraphBuilder) *GraphStore {
	if builder == nil {
		builder = &DefaultBuilder{}
	}
	return &GraphStore{
		repo:    repo,
		builder: builder,
	}
}

func (s *GraphStore) AppendEvent(ctx context.Context, c *Chat, event Event) error {
	node, err := s.builder.Build(ctx, c, event)
	if err != nil {
		return fmt.Errorf("failed to build node from event: %w", err)
	}
	return s.repo.AppendNode(ctx, c, node)
}

func (s *GraphStore) SaveDigest(ctx context.Context, c *Chat, digest DigestMeta) error {
	return s.repo.SaveDigest(ctx, c, digest)
}

func (s *GraphStore) SaveSpaceContext(ctx context.Context, c *Chat, spaceCtx SpaceContext) error {
	return s.repo.SaveSpaceContext(ctx, c, spaceCtx)
}

func (s *GraphStore) SaveMutation(ctx context.Context, c *Chat, mutation MutationEvent) error {
	return s.repo.SaveMutation(ctx, c, mutation)
}

// Repository exposes the underlying repository for direct reads and state management.
func (s *GraphStore) Repository() GraphRepository {
	return s.repo
}

// GraphRepository defines the fast KV index for non-linear traversal. It stores stateful Nodes.
type GraphRepository interface {
	AppendNode(ctx context.Context, c *Chat, node Node) error
	SaveDigest(ctx context.Context, c *Chat, digest DigestMeta) error
	SaveSpaceContext(ctx context.Context, c *Chat, spaceCtx SpaceContext) error
	SaveMutation(ctx context.Context, c *Chat, mutation MutationEvent) error

	ClearChatGraph(ctx context.Context, c *Chat) error
	Close() error

	GetSyncState(ctx context.Context, c *Chat) (*LedgerManifest, error)
	SetSyncState(ctx context.Context, c *Chat, manifest LedgerManifest) error

	// Single-branch reads
	GetBranchTimeline(ctx context.Context, c *Chat, branchID string) ([]Node, error)
	GetBranchTimelinePage(ctx context.Context, c *Chat, branchID string, cursor string, limit int) ([]Node, string, bool, error)
	WalkBranchReverse(ctx context.Context, c *Chat, branchID string, startCursor string, fn func(Node) bool) error

	GetLenses(ctx context.Context, c *Chat, tag string) (LensResult, error)
	GetStickyDigests(ctx context.Context, c *Chat) ([]DigestMeta, error)
	GetDigests(ctx context.Context, c *Chat) ([]DigestMeta, error)
	GetSpaceContexts(ctx context.Context, c *Chat) ([]SpaceContext, error)
	GetNodes(ctx context.Context, c *Chat, ids []string) ([]Node, error)
}
