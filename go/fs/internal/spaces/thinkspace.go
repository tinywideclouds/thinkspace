package spaces

import (
	"context"

	"github.com/tinywideclouds.com/thinkspace/internal/assembler"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type ThinkSpace interface {
	Config() ThinkSpaceConfig
	Tools() []llm.ToolDef
	Verifier() Verifier
	Mapbook() assembler.Mapbook

	// Patcher provides the domain's specific file-modification engine.
	Patcher() workspace.Patcher

	Initialize(ctx context.Context, directory string) error
}
