package golang

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces/generic"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// Options holds Go-domain-specific settings extracted from ThinkStateConfig.Options.
type Options struct {
	FormatImports bool
}

func parseOptions(raw map[string]any) Options {
	opts := Options{
		FormatImports: true, // Enabled by default in Go ThinkSpaces
	}
	if raw == nil {
		return opts
	}
	if v, ok := raw["format_imports"].(bool); ok {
		opts.FormatImports = v
	}
	return opts
}

// GoThinkState implements spaces.ThinkSpace for the Go programming language.
type GoThinkState struct {
	config  spaces.ThinkStateConfig
	options Options
}

func NewGoThinkState(cfg spaces.ThinkStateConfig) *GoThinkState {
	return &GoThinkState{
		config:  cfg,
		options: parseOptions(cfg.Options),
	}
}

func (s *GoThinkState) Config() spaces.ThinkStateConfig {
	return s.config
}

func (s *GoThinkState) Options() Options {
	return s.options
}

func (s *GoThinkState) Verifier() spaces.Verifier {
	return generic.NewCIVerifier(s.config.Toolchain, s.config.VerifyTimeout())
}

func (s *GoThinkState) Mapbook() spaces.Mapbook {
	return NewGolangMapbook()
}

func (s *GoThinkState) Patcher() workspace.Patcher {
	return NewGoASTPatcher(s.config.Roles.Worker.PatcherInstructions, s.options.FormatImports)
}

func (s *GoThinkState) Initialize(ctx context.Context, directory string) error {
	srcDir := filepath.Join(directory, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		return fmt.Errorf("scaffolding src directory: %w", err)
	}

	goModPath := filepath.Join(srcDir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		cmd := exec.CommandContext(ctx, "go", "mod", "init", "thinkspace/app")
		cmd.Dir = srcDir
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("initializing go module: %w", err)
		}
	}
	return nil
}

func (g *GoThinkState) EnrichBundle(ctx context.Context, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus, workspaceRoot string) error {
	requiresDependencies := false

	// Check if the current cognitive focus explicitly targets dependencies
	for _, tag := range focus.PrimaryFocus {
		if tag == "dependencies" || tag == "module" {
			requiresDependencies = true
			break
		}
	}
	if !requiresDependencies {
		for _, tag := range focus.RelatedContext {
			if tag == "dependencies" || tag == "module" {
				requiresDependencies = true
				break
			}
		}
	}

	// If dependencies are in focus, proactively load the go.mod file into the Workbench
	if requiresDependencies {
		goModPath := filepath.Join(workspaceRoot, "src", "go.mod")
		content, err := os.ReadFile(goModPath)
		if err == nil {
			bundle.Workbench = append(bundle.Workbench, cognition.WorkbenchItem{
				Path:    "src/go.mod",
				Content: string(content),
			})
		}
	}

	return nil
}
