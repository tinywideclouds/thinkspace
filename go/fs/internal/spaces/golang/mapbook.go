package golang

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
)

type GolangMapbook struct{}

func NewGolangMapbook() *GolangMapbook {
	return &GolangMapbook{}
}

func (g *GolangMapbook) GenerateLayers(ctx context.Context, workspaceRoot string) (map[spaces.MapbookLayer]string, error) {
	layers := make(map[spaces.MapbookLayer]string)

	var treeBuilder strings.Builder
	treeBuilder.WriteString("Directory Tree:\n")

	srcPath := filepath.Join(workspaceRoot, "src")
	err := filepath.WalkDir(srcPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if path == srcPath && errors.Is(walkErr, os.ErrNotExist) {
				return filepath.SkipAll
			}
			return walkErr
		}

		if d.IsDir() && path != srcPath && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}

		rel, relErr := filepath.Rel(workspaceRoot, path)
		if relErr != nil {
			return fmt.Errorf("computing relative path for %s: %w", path, relErr)
		}

		if d.IsDir() {
			treeBuilder.WriteString(fmt.Sprintf("- %s/\n", filepath.ToSlash(rel)))
		} else {
			treeBuilder.WriteString(fmt.Sprintf("  - %s\n", filepath.ToSlash(rel)))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking Go source tree: %w", err)
	}

	layers[spaces.LayerStructural] = treeBuilder.String()
	layers[spaces.LayerConceptual] = "Architecture Rules: Standard Go project layout. All Go code must reside in the src/ directory."

	return layers, nil
}
