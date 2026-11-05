package gemini

import (
	"google.golang.org/genai"

	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
)

func CompileProposeChange(tool *tools.ProposeChangeTool, config spaces.ThinkStateConfig) *genai.FunctionDeclaration {
	description := tool.DefaultDescription()
	agentCount := "Number of parallel sub-agents to spawn."
	contextDigest := "Contextual background explaining why this work is needed."
	instruction := "Clear instructions detailing what changes or work to execute."
	targetFiles := "Existing files that the worker must consult or edit."

	if config.Roles.Manager.Tools.ProposeChange.Description != "" {
		description = config.Roles.Manager.Tools.ProposeChange.Description
	}
	if config.Roles.Manager.Tools.ProposeChange.AgentCountDescription != "" {
		agentCount = config.Roles.Manager.Tools.ProposeChange.AgentCountDescription
	}
	if config.Roles.Manager.Tools.ProposeChange.ContextDigestDescription != "" {
		contextDigest = config.Roles.Manager.Tools.ProposeChange.ContextDigestDescription
	}
	if config.Roles.Manager.Tools.ProposeChange.InstructionDescription != "" {
		instruction = config.Roles.Manager.Tools.ProposeChange.InstructionDescription
	}
	if config.Roles.Manager.Tools.ProposeChange.TargetFilesDescription != "" {
		targetFiles = config.Roles.Manager.Tools.ProposeChange.TargetFilesDescription
	}

	return &genai.FunctionDeclaration{
		Name:        tool.Name().String(),
		Description: description,
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				tools.Instruction: {
					Type:        genai.TypeString,
					Description: instruction,
				},
				tools.ContextDigest: {
					Type:        genai.TypeString,
					Description: contextDigest,
				},
				tools.TargetFiles: {
					Type:        genai.TypeArray,
					Items:       &genai.Schema{Type: genai.TypeString},
					Description: targetFiles,
				},
				tools.AgentCount: {
					Type:        genai.TypeInteger,
					Description: agentCount,
				},
			},
			Required: []string{tools.Instruction, tools.ContextDigest},
		},
	}
}
