package golang

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tinywideclouds.com/thinkspace/internal/assembler"
)

type GolangMapbook struct{}

func NewGolangMapbook() *GolangMapbook {
	return &GolangMapbook{}
}

func (g *GolangMapbook) GenerateLayers(ctx context.Context, workspaceRoot string) (map[assembler.MapbookLayer]string, error) {
	layers := make(map[assembler.MapbookLayer]string)

	var treeBuilder strings.Builder
	treeBuilder.WriteString("Directory Tree:\n")

	srcPath := filepath.Join(workspaceRoot, "src")
	err := filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip missing dirs gracefully
		}
		rel, _ := filepath.Rel(workspaceRoot, path)
		if info.IsDir() {
			treeBuilder.WriteString(fmt.Sprintf("- %s/\n", filepath.ToSlash(rel)))
		} else {
			treeBuilder.WriteString(fmt.Sprintf("  - %s\n", filepath.ToSlash(rel)))
		}
		return nil
	})

	if err == nil {
		layers[assembler.LayerStructural] = treeBuilder.String()
	}

	layers[assembler.LayerConceptual] = "Architecture Rules: Standard Go project layout. All Go code must reside in the src/ directory."

	return layers, nil
}
