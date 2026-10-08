package golang

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/tinywideclouds.com/thinkspace/internal/assembler"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// GoThinkSpace implements spaces.ThinkSpace for the Go programming language.
type GoThinkSpace struct {
	config spaces.ThinkSpaceConfig
}

func NewGoThinkSpace(cfg spaces.ThinkSpaceConfig) *GoThinkSpace {
	return &GoThinkSpace{
		config: cfg,
	}
}

func (s *GoThinkSpace) Config() spaces.ThinkSpaceConfig {
	return s.config
}

func (s *GoThinkSpace) Tools() []llm.ToolDef {
	managerTools := s.config.Roles.Manager.Tools

	return []llm.ToolDef{
		{
			Name:        "propose_change",
			Description: managerTools.ProposeChange.Description,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"assigned_tags": map[string]any{
						"type":        "array",
						"description": managerTools.ProposeChange.AssignedTagsDescription,
						"items":       map[string]any{"type": "string"},
					},
					"agent_count": map[string]any{
						"type":        "integer",
						"description": managerTools.ProposeChange.AgentCountDescription,
					},
					"agent_tasks": map[string]any{
						"type":        "array",
						"description": managerTools.ProposeChange.AgentInstructionsDescription,
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"context_digest": map[string]any{"type": "string", "description": managerTools.ProposeChange.ContextDigestDescription},
								"instruction":    map[string]any{"type": "string", "description": managerTools.ProposeChange.InstructionDescription},
								"target_files": map[string]any{
									"type":        "array",
									"description": managerTools.ProposeChange.TargetFilesDescription,
									"items":       map[string]any{"type": "string"},
								},
							},
							"required": []string{"context_digest", "instruction", "target_files"},
						},
					},
				},
			},
			Required: []string{"assigned_tags", "agent_count", "agent_tasks"},
		},
		{
			Name:        "query_lens",
			Description: managerTools.QueryLens.Description,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"tag":       map[string]any{"type": "string", "description": managerTools.QueryLens.TagDescription},
					"reasoning": map[string]any{"type": "string", "description": managerTools.QueryLens.ReasoningDescription},
				},
			},
			Required: []string{"tag", "reasoning"},
		},
	}
}

func (s *GoThinkSpace) Verifier() spaces.Verifier {
	return NewGoVerifier(s.config.TurnTimeout())
}

func (s *GoThinkSpace) Mapbook() assembler.Mapbook {
	return NewGolangMapbook()
}

func (s *GoThinkSpace) Patcher() workspace.Patcher {
	return NewGoASTPatcher(s.config.Roles.Worker.PatcherInstructions)
}

func (s *GoThinkSpace) Initialize(ctx context.Context, directory string) error {
	srcDir := filepath.Join(directory, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		return fmt.Errorf("scaffolding src directory: %w", err)
	}

	// If a go.mod doesn't exist, generate a default one
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
