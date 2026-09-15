package golang

import (
	"github.com/tinywideclouds.com/thinkspace/internal/assembler"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"google.golang.org/genai"
)

// GoThinkSpace implements spaces.ThinkSpace for the Go programming language.
type GoThinkSpace struct {
	config spaces.ThinkSpaceConfig
}

// NewGoThinkSpace injects the externalized YAML configuration.
func NewGoThinkSpace(cfg spaces.ThinkSpaceConfig) *GoThinkSpace {
	return &GoThinkSpace{
		config: cfg,
	}
}

func (s *GoThinkSpace) Config() spaces.ThinkSpaceConfig {
	return s.config
}

func (s *GoThinkSpace) Tools() []*genai.Tool {
	managerTools := s.config.Roles.Manager.Tools

	return []*genai.Tool{
		{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Name:        "propose_change",
					Description: managerTools.ProposeChange.Description,
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"assigned_tags": {
								Type:        genai.TypeArray,
								Description: managerTools.ProposeChange.AssignedTagsDescription,
								Items: &genai.Schema{
									Type: genai.TypeString,
								},
							},
							"agent_count": {
								Type:        genai.TypeInteger,
								Description: managerTools.ProposeChange.AgentCountDescription,
							},
							"agent_tasks": {
								Type:        genai.TypeArray,
								Description: managerTools.ProposeChange.AgentInstructionsDescription,
								Items: &genai.Schema{
									Type: genai.TypeObject,
									Properties: map[string]*genai.Schema{
										"context_digest": {
											Type:        genai.TypeString,
											Description: managerTools.ProposeChange.ContextDigestDescription,
										},
										"instruction": {
											Type:        genai.TypeString,
											Description: managerTools.ProposeChange.InstructionDescription,
										},
										"target_files": {
											Type:        genai.TypeArray,
											Description: managerTools.ProposeChange.TargetFilesDescription,
											Items: &genai.Schema{
												Type: genai.TypeString,
											},
										},
									},
									Required: []string{"context_digest", "instruction", "target_files"},
								},
							},
						},
						Required: []string{"assigned_tags", "agent_count", "agent_tasks"},
					},
				},
				{
					Name:        "query_lens",
					Description: managerTools.QueryLens.Description,
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"tag": {
								Type:        genai.TypeString,
								Description: managerTools.QueryLens.TagDescription,
							},
							"reasoning": {
								Type:        genai.TypeString,
								Description: managerTools.QueryLens.ReasoningDescription,
							},
						},
						Required: []string{"tag", "reasoning"},
					},
				},
			},
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
