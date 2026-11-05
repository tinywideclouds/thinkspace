package gemini

import (
	"google.golang.org/genai"

	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
)

func CompileTransitionThinkState(tool *tools.TransitionThinkStateTool) *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        tool.Name().String(),
		Description: tool.DefaultDescription(),
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				tools.TargetThinkState: {
					Type:        genai.TypeString,
					Enum:        tool.ValidStates(),
					Description: "The identifier of the target think state to enter.",
				},
				tools.Reasoning: {
					Type:        genai.TypeString,
					Description: "The architectural rationale for requesting this transition.",
				},
			},
			Required: []string{tools.TargetThinkState, tools.Reasoning},
		},
	}
}
