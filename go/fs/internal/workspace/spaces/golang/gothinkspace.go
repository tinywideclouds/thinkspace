package golang

import (
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"google.golang.org/genai"
)

// GoThinkSpace implements ThinkSpace for the Go programming language.
type GoThinkSpace struct {
	config workspace.ThinkSpaceConfig
}

// NewGoThinkSpace injects the externalized YAML configuration.
func NewGoThinkSpace(cfg workspace.ThinkSpaceConfig) *GoThinkSpace {
	return &GoThinkSpace{
		config: cfg,
	}
}

func (s *GoThinkSpace) Config() workspace.ThinkSpaceConfig {
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
									},
									Required: []string{"context_digest", "instruction"},
								},
							},
						},
						Required: []string{"agent_count", "agent_tasks"},
					},
				},
			},
		},
	}
}

// Verifier returns the domain-specific verification engine.
func (s *GoThinkSpace) Verifier() workspace.Verifier {
	return NewGoVerifier(s.config.VerifyTimeout())
}
