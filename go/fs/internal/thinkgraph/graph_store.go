package thinkgraph

import (
	"context"
	"fmt"
)

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

func (s *GraphStore) AppendEvent(ctx context.Context, c *ChatAddress, event Event) error {
	node, err := s.builder.Build(ctx, c, event)
	if err != nil {
		return fmt.Errorf("failed to build node from event: %w", err)
	}
	return s.repo.AppendNode(ctx, c, node)
}

func (s *GraphStore) SaveDigest(ctx context.Context, c *ChatAddress, digest DigestMeta) error {
	return s.repo.SaveDigest(ctx, c, digest)
}

func (s *GraphStore) SaveSpaceContext(ctx context.Context, c *ChatAddress, spaceCtx SpaceContext) error {
	return s.repo.SaveSpaceContext(ctx, c, spaceCtx)
}

func (s *GraphStore) SaveMutation(ctx context.Context, c *ChatAddress, mutation Mutator) error {
	return s.repo.SaveMutation(ctx, c, mutation)
}

func (s *GraphStore) Repository() GraphRepository {
	return s.repo
}

type GraphRepository interface {
	AppendNode(ctx context.Context, c *ChatAddress, node Node) error
	SaveDigest(ctx context.Context, c *ChatAddress, digest DigestMeta) error
	SaveSpaceContext(ctx context.Context, c *ChatAddress, spaceCtx SpaceContext) error
	SaveMutation(ctx context.Context, c *ChatAddress, mutation Mutator) error

	ClearChatGraph(ctx context.Context, c *ChatAddress) error
	Close() error

	GetSyncState(ctx context.Context, c *ChatAddress) (*LedgerManifest, error)
	SetSyncState(ctx context.Context, c *ChatAddress, manifest LedgerManifest) error

	// SubGraph reads
	GetSubGraphTimeline(ctx context.Context, c *ChatAddress, graphID GraphID) ([]Node, error)
	GetSubGraphTimelinePage(ctx context.Context, c *ChatAddress, graphID GraphID, cursor string, limit int) ([]Node, string, bool, error)
	WalkSubGraphReverse(ctx context.Context, c *ChatAddress, graphID GraphID, startCursor string, fn func(Node) bool) error

	GetLenses(ctx context.Context, c *ChatAddress, tag string) (LensResult, error)
	GetStickyDigests(ctx context.Context, c *ChatAddress) ([]DigestMeta, error)
	GetDigests(ctx context.Context, c *ChatAddress) ([]DigestMeta, error)
	GetSpaceContexts(ctx context.Context, c *ChatAddress) ([]SpaceContext, error)
	GetNodes(ctx context.Context, c *ChatAddress, ids []string) ([]Node, error)
}
