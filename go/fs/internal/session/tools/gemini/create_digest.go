package gemini

import (
	"google.golang.org/genai"

	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
)

func CompileCreateDigest(tool *tools.CreateDigestTool, config spaces.ThinkStateConfig) *genai.FunctionDeclaration {
	description := tool.DefaultDescription()
	topicName := "Short descriptive name of the topic summarized."
	instruction := "Guidance on critical constraints or facts that must be preserved."
	tags := "Associated classification tags for future lens lookup."
	isSticky := "Pin this digest unconditionally in every turn's context regardless of topic focus. Leave false for normal tag-sensitive digests."

	if config.Roles.Manager.Tools.CreateDigest.Description != "" {
		description = config.Roles.Manager.Tools.CreateDigest.Description
	}
	if config.Roles.Manager.Tools.CreateDigest.TopicNameDescription != "" {
		topicName = config.Roles.Manager.Tools.CreateDigest.TopicNameDescription
	}
	if config.Roles.Manager.Tools.CreateDigest.InstructionDescription != "" {
		instruction = config.Roles.Manager.Tools.CreateDigest.InstructionDescription
	}
	if config.Roles.Manager.Tools.CreateDigest.TagsDescription != "" {
		tags = config.Roles.Manager.Tools.CreateDigest.TagsDescription
	}
	if config.Roles.Manager.Tools.CreateDigest.IsStickyDescription != "" {
		isSticky = config.Roles.Manager.Tools.CreateDigest.IsStickyDescription
	}

	return &genai.FunctionDeclaration{
		Name:        tool.Name().String(),
		Description: description,
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				tools.TopicName: {
					Type:        genai.TypeString,
					Description: topicName,
				},
				tools.DigestInstruction: {
					Type:        genai.TypeString,
					Description: instruction,
				},
				tools.Tags: {
					Type:        genai.TypeArray,
					Items:       &genai.Schema{Type: genai.TypeString},
					Description: tags,
				},
				tools.IsSticky: {
					Type:        genai.TypeBoolean,
					Description: isSticky,
				},
			},
			Required: []string{tools.TopicName, tools.DigestInstruction},
		},
	}
}
