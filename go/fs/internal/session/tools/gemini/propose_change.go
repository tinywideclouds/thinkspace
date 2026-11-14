package gemini

import (
	"google.golang.org/genai"

	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
)

func CompileProposeChange(tool *tools.ProposeChangeTool, config spaces.ThinkStateConfig) *genai.FunctionDeclaration {
	description := tool.DefaultDescription()
	agentCount := "Number of parallel sub-agents to spawn."
	agentInstructions := "An array of explicit instruction objects, exactly matching the agent_count."
	contextDigest := "Contextual background explaining why this work is needed."
	instruction := "Clear instructions detailing what changes or work to execute."
	targetFiles := "Existing files that the worker must consult or edit."

	if config.Roles.Manager.Tools.ProposeChange.Description != "" {
		description = config.Roles.Manager.Tools.ProposeChange.Description
	}
	if config.Roles.Manager.Tools.ProposeChange.AgentCountDescription != "" {
		agentCount = config.Roles.Manager.Tools.ProposeChange.AgentCountDescription
	}
	// Fallback description as the field doesn't exist yet on spaces.ThinkStateConfig
	// if config.Roles.Manager.Tools.ProposeChange.AgentInstructionsDescription != "" {
	// 	agentInstructions = config.Roles.Manager.Tools.ProposeChange.AgentInstructionsDescription
	// }
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
				tools.AgentInstructions: {
					Type:        genai.TypeArray,
					Description: agentInstructions,
					Items: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							tools.Approach: {
								Type:        genai.TypeString,
								Description: "Short label identifying this architectural or algorithmic approach.",
							},
							tools.Instruction: {
								Type:        genai.TypeString,
								Description: "Distinct, self-contained implementation instructions for this sub-agent.",
							},
						},
						Required: []string{tools.Approach, tools.Instruction},
					},
				},
			},
			Required: []string{tools.ContextDigest, tools.AgentCount, tools.AgentInstructions},
		},
	}
}
