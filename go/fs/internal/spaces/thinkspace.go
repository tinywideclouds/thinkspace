package spaces

import (
	"github.com/tinywideclouds.com/thinkspace/internal/assembler"
	"google.golang.org/genai"
)

// ThinkSpace defines the highly stable contract for a domain-specific environment.
// It sits above the workspace infrastructure, orchestrating tools and context.
type ThinkSpace interface {
	Config() ThinkSpaceConfig
	Tools() []*genai.Tool

	// Verifier returns the domain-specific verification engine.
	Verifier() Verifier

	// Mapbook provides the domain-specific logic for parsing workspace layers.
	Mapbook() assembler.Mapbook
}
