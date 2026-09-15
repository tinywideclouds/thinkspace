package spaces

import (
	"github.com/tinywideclouds.com/thinkspace/internal/assembler"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"google.golang.org/genai"
)

type ThinkSpace interface {
	Config() ThinkSpaceConfig
	Tools() []*genai.Tool
	Verifier() Verifier
	Mapbook() assembler.Mapbook

	// Patcher provides the domain's specific file-modification engine.
	Patcher() workspace.Patcher
}
