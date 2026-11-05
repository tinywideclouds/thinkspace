package spaces

import (
	"context"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type ThinkState interface {
	Config() ThinkStateConfig
	Verifier() Verifier
	Mapbook() Mapbook
	Patcher() workspace.Patcher

	Initialize(ctx context.Context, directory string) error
	EnrichBundle(ctx context.Context, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus, workspaceRoot string) error
}
