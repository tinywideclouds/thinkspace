package concept

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces/generic"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type ConceptState struct {
	config spaces.ThinkStateConfig
}

func NewConceptState(cfg spaces.ThinkStateConfig) *ConceptState {
	return &ConceptState{config: cfg}
}

func (s *ConceptState) Config() spaces.ThinkStateConfig {
	return s.config
}

func (s *ConceptState) Tools() []llm.ToolDef {
	managerTools := s.config.Roles.Manager.Tools

	return []llm.ToolDef{
		{
			Name:        "propose_change",
			Description: managerTools.ProposeChange.Description,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"assigned_tags": map[string]any{"type": "array", "description": managerTools.ProposeChange.AssignedTagsDescription, "items": map[string]any{"type": "string"}},
					"agent_count":   map[string]any{"type": "integer", "description": managerTools.ProposeChange.AgentCountDescription},
					"agent_tasks": map[string]any{
						"type": "array", "description": managerTools.ProposeChange.AgentInstructionsDescription,
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"context_digest": map[string]any{"type": "string", "description": managerTools.ProposeChange.ContextDigestDescription},
								"instruction":    map[string]any{"type": "string", "description": managerTools.ProposeChange.InstructionDescription},
								"target_files":   map[string]any{"type": "array", "description": managerTools.ProposeChange.TargetFilesDescription, "items": map[string]any{"type": "string"}},
							},
							"required": []string{"context_digest", "instruction", "target_files"},
						},
					},
				},
			},
			Required: []string{"assigned_tags", "agent_count", "agent_tasks"},
		},
		{
			Name:        "query_lens",
			Description: managerTools.QueryLens.Description,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"tag":       map[string]any{"type": "string", "description": managerTools.QueryLens.TagDescription},
					"reasoning": map[string]any{"type": "string", "description": managerTools.QueryLens.ReasoningDescription},
				},
			},
			Required: []string{"tag", "reasoning"},
		},
		{
			Name:        "create_digest",
			Description: managerTools.CreateDigest.Description,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"topic_name":  map[string]any{"type": "string", "description": managerTools.CreateDigest.TopicNameDescription},
					"tags":        map[string]any{"type": "array", "description": managerTools.CreateDigest.TagsDescription, "items": map[string]any{"type": "string"}},
					"instruction": map[string]any{"type": "string", "description": managerTools.CreateDigest.InstructionDescription},
				},
			},
			Required: []string{"topic_name", "tags", "instruction"},
		},
	}
}

func (s *ConceptState) Verifier() spaces.Verifier {
	return &generic.NoOpVerifier{}
}

func (s *ConceptState) Mapbook() spaces.Mapbook {
	return NewConceptMapbook()
}

func (s *ConceptState) Patcher() workspace.Patcher {
	return generic.NewSearchReplacePatcher(s.config.Roles.Worker.PatcherInstructions)
}

func (s *ConceptState) Initialize(ctx context.Context, directory string) error {
	// Prose spaces don't have compilers, but standardizing a docs/ folder is helpful
	docsDir := filepath.Join(directory, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		return fmt.Errorf("scaffolding docs directory: %w", err)
	}
	return nil
}

func (s *ConceptState) EnrichBundle(ctx context.Context, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus, workspaceRoot string) error {
	// The Concept state does not require strict file pre-loading like Golang does with go.mod.
	// This hook is available if we later want to automatically inject domain-specific prose or roadmap files based on focus tags.
	return nil
}
