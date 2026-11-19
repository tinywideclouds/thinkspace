package gemini

import (
	"google.golang.org/genai"

	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
)

func CompileProposeChange(tool *tools.ProposeChangeTool, config spaces.ThinkStateConfig) *genai.FunctionDeclaration {
	description := tool.DefaultDescription()
	intention := "Concise, user-facing explanation of why you are spawning sub-agents and what approaches they will explore."
	assignedTags := "Semantic tags categorizing this work (reuse existing tags from active context whenever applicable)."
	agentCount := "Number of parallel sub-agents to spawn."
	agentInstructions := "An array of explicit instruction objects, exactly matching the agent_count."
	contextDigest := "Contextual background explaining why this work is needed."
	instruction := "Clear instructions detailing what changes or work to execute."
	targetFiles := "Existing files that the worker must consult or edit."

	if config.Roles.Manager.Tools.ProposeChange.Description != "" {
		description = config.Roles.Manager.Tools.ProposeChange.Description
	}
	if config.Roles.Manager.Tools.ProposeChange.IntentionDescription != "" {
		intention = config.Roles.Manager.Tools.ProposeChange.IntentionDescription
	}
	if config.Roles.Manager.Tools.ProposeChange.AssignedTagsDescription != "" {
		assignedTags = config.Roles.Manager.Tools.ProposeChange.AssignedTagsDescription
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
				tools.Intention: {
					Type:        genai.TypeString,
					Description: intention,
				},
				tools.AssignedTags: {
					Type:        genai.TypeArray,
					Items:       &genai.Schema{Type: genai.TypeString},
					Description: assignedTags,
				},
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
			Required: []string{tools.Intention, tools.AssignedTags, tools.ContextDigest, tools.AgentCount, tools.AgentInstructions},
		},
	}
}
