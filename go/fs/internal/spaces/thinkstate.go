package spaces

import (
	"context"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type ThinkState interface {
	Config() ThinkStateConfig
	Tools() []llm.ToolDef
	Verifier() Verifier
	Mapbook() Mapbook
	Patcher() workspace.Patcher

	// Lifecycle
	Initialize(ctx context.Context, directory string) error

	// Cognitive Hook
	EnrichBundle(ctx context.Context, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus, workspaceRoot string) error
}
