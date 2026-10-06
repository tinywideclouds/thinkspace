package golang

import (
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
