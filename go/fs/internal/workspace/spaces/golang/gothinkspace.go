package golang

import (
	"time"

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

func (s *GoThinkSpace) Name() string {
	return s.config.Name
}

func (s *GoThinkSpace) SystemPrompt() string {
	return s.config.SystemPrompt
}

func (s *GoThinkSpace) SubAgentSystemPrompt() string {
	return s.config.SubAgentSystemPrompt
}

func (s *GoThinkSpace) Model(category workspace.ModelCategory) string {
	return s.config.Models[category]
}

func (s *GoThinkSpace) TurnTimeout() time.Duration {
	return time.Duration(s.config.TurnTimeoutSeconds) * time.Second
}

func (s *GoThinkSpace) AgentTimeout() time.Duration {
	return time.Duration(s.config.AgentTimeoutSeconds) * time.Second
}

func (s *GoThinkSpace) VerifyTimeout() time.Duration {
	return time.Duration(s.config.VerifyTimeoutSeconds) * time.Second
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
							"agent_instructions": {
								Type:        genai.TypeArray,
								Description: s.config.AgentInstructionsDescription,
								Items: &genai.Schema{
									Type: genai.TypeString,
								},
							},
						},
						Required: []string{"agent_count", "agent_instructions"},
					},
				},
			},
		},
	}
}

// Verifier returns the domain-specific verification engine.
func (s *GoThinkSpace) Verifier() workspace.Verifier {
	return NewGoVerifier(s.VerifyTimeout())
}
