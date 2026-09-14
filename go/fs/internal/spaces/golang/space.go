package golang

import (
	"github.com/tinywideclouds.com/thinkspace/internal/assembler"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
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
	return []*genai.Tool{
		{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Name:        "propose_change",
					Description: s.config.ToolDescription,
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"assigned_tags": {
								Type:        genai.TypeArray,
								Description: s.config.AssignedTagsDescription,
								Items: &genai.Schema{
									Type: genai.TypeString,
								},
							},
							"agent_count": {
								Type:        genai.TypeInteger,
								Description: s.config.AgentCountDescription,
							},
							"agent_tasks": {
								Type:        genai.TypeArray,
								Description: s.config.AgentInstructionsDescription,
								Items: &genai.Schema{
									Type: genai.TypeObject,
									Properties: map[string]*genai.Schema{
										"context_digest": {
											Type:        genai.TypeString,
											Description: s.config.ContextDigestDescription,
										},
										"instruction": {
											Type:        genai.TypeString,
											Description: s.config.InstructionDescription,
										},
										"target_files": {
											Type:        genai.TypeArray,
											Description: s.config.TargetFilesDescription,
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
			},
		},
	}
}

func (s *GoThinkSpace) Verifier() spaces.Verifier {
	return NewGoVerifier(s.config.VerifyTimeout())
}

func (s *GoThinkSpace) Mapbook() assembler.Mapbook {
	return NewGolangMapbook()
}
