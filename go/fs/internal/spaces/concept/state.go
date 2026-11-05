package concept

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces/generic"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type ConceptState struct {
	config spaces.ThinkStateConfig
}

func NewConceptState(cfg spaces.ThinkStateConfig) *ConceptState {
	return &ConceptState{config: cfg}
}

func (s *ConceptState) Config() spaces.ThinkStateConfig {
	return s.config
}

func (s *ConceptState) Verifier() spaces.Verifier {
	return &generic.NoOpVerifier{}
}

func (s *ConceptState) Mapbook() spaces.Mapbook {
	return NewConceptMapbook()
}

func (s *ConceptState) Patcher() workspace.Patcher {
	return generic.NewSearchReplacePatcher(s.config.Roles.Worker.PatcherInstructions)
}

func (s *ConceptState) Initialize(ctx context.Context, directory string) error {
	// Prose spaces don't have compilers, but standardizing a docs/ folder is helpful
	docsDir := filepath.Join(directory, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		return fmt.Errorf("scaffolding docs directory: %w", err)
	}
	return nil
}

func (s *ConceptState) EnrichBundle(ctx context.Context, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus, workspaceRoot string) error {
	// The Concept state does not require strict file pre-loading like Golang does with go.mod.
	// This hook is available if we later want to automatically inject domain-specific prose or roadmap files based on focus tags.
	return nil
}
