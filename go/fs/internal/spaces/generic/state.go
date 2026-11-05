package generic

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type GenericMapbook struct{}

func (m *GenericMapbook) GenerateLayers(ctx context.Context, workspaceRoot string) (map[spaces.MapbookLayer]string, error) {
	layers := make(map[spaces.MapbookLayer]string)
	layers[spaces.LayerConceptual] = "Workspace Overview: Generic ThinkState."
	return layers, nil
}

// GenericThinkState provides a baseline ThinkState implementation for non-code or lightweight environments.
type GenericThinkState struct {
	config spaces.ThinkStateConfig
}

func NewGenericThinkState(cfg spaces.ThinkStateConfig) *GenericThinkState {
	return &GenericThinkState{config: cfg}
}

func (s *GenericThinkState) Config() spaces.ThinkStateConfig {
	return s.config
}

func (s *GenericThinkState) Verifier() spaces.Verifier {
	return NewNoOpVerifier()
}

func (s *GenericThinkState) Mapbook() spaces.Mapbook {
	return &GenericMapbook{}
}

func (s *GenericThinkState) Patcher() workspace.Patcher {
	return NewSearchReplacePatcher(s.config.Roles.Worker.PatcherInstructions)
}

func (s *GenericThinkState) Initialize(ctx context.Context, directory string) error {
	docsDir := filepath.Join(directory, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		return fmt.Errorf("scaffolding docs directory: %w", err)
	}
	return nil
}

func (s *GenericThinkState) EnrichBundle(ctx context.Context, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus, workspaceRoot string) error {
	return nil
}
