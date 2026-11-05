package gemini

import (
	"google.golang.org/genai"

	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
)

func CompileQueryLens(tool *tools.QueryLensTool, config spaces.ThinkStateConfig) *genai.FunctionDeclaration {
	description := tool.DefaultDescription()
	tag := "The tag to query from memory (e.g. 'architecture', 'auth')."
	reasoning := "Why this context is needed for the current turn."

	if config.Roles.Manager.Tools.QueryLens.Description != "" {
		description = config.Roles.Manager.Tools.QueryLens.Description
	}
	if config.Roles.Manager.Tools.QueryLens.TagDescription != "" {
		tag = config.Roles.Manager.Tools.QueryLens.TagDescription
	}
	if config.Roles.Manager.Tools.QueryLens.ReasoningDescription != "" {
		reasoning = config.Roles.Manager.Tools.QueryLens.ReasoningDescription
	}

	return &genai.FunctionDeclaration{
		Name:        tool.Name().String(),
		Description: description,
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				tools.Tag: {
					Type:        genai.TypeString,
					Description: tag,
				},
				tools.Reasoning: {
					Type:        genai.TypeString,
					Description: reasoning,
				},
			},
			Required: []string{tools.Tag},
		},
	}
}