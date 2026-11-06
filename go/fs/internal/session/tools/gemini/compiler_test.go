package gemini_test

import (
	"testing"

	"google.golang.org/genai"

	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools/gemini"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
)

func TestCompileCoreTools_OverridesDescriptionsNotSchema(t *testing.T) {
	// Provide a config that aggressively attempts to override descriptions
	cfg := spaces.ThinkStateConfig{
		Roles: spaces.RolesConfig{
			Manager: spaces.ManagerConfig{
				Tools: spaces.ToolsConfig{
					ProposeChange: spaces.ToolProposeChangeConfig{
						Description:            "Custom Propose",
						InstructionDescription: "Custom Instruction",
					},
					QueryLens: spaces.ToolQueryLensConfig{
						Description: "Custom Query",
					},
					CreateDigest: spaces.ToolCreateDigestConfig{
						Description: "Custom Digest",
					},
				},
			},
		},
	}

	transitionTool := tools.NewTransitionThinkStateTool([]string{"golang", "concept"})
	proposeTool := tools.NewProposeChangeTool()
	queryTool := tools.NewQueryLensTool(nil)
	digestTool := tools.NewCreateDigestTool(nil)

	genaiTools := gemini.CompileCoreTools(transitionTool, proposeTool, queryTool, digestTool, cfg)

	if len(genaiTools) != 1 {
		t.Fatalf("Expected 1 GenAI Tool object, got %d", len(genaiTools))
	}

	decls := genaiTools[0].FunctionDeclarations
	if len(decls) != 4 {
		t.Fatalf("Expected 4 function declarations, got %d", len(decls))
	}

	// 1. Check Transition
	transitionDecl := decls[0]
	if transitionDecl.Name != "transition_workspace" {
		t.Errorf("Expected 'transition_workspace', got %s", transitionDecl.Name)
	}
	targetProp := transitionDecl.Parameters.Properties[tools.TargetThinkState]
	if targetProp.Type != genai.TypeString {
		t.Errorf("Schema lock failed: expected string type for target_think_state")
	}
	if len(targetProp.Enum) != 2 || targetProp.Enum[0] != "golang" {
		t.Errorf("Enum injection failed for transition tool")
	}

	// 2. Check ProposeChange
	proposeDecl := decls[1]
	if proposeDecl.Description != "Custom Propose" {
		t.Errorf("YAML config failed to override ProposeChange description")
	}
	instructionProp := proposeDecl.Parameters.Properties[tools.Instruction]
	if instructionProp.Description != "Custom Instruction" {
		t.Errorf("YAML config failed to override property description")
	}
	if instructionProp.Type != genai.TypeString {
		t.Errorf("Schema lock failed: expected string type for instruction")
	}

	// 3. Check QueryLens
	queryDecl := decls[2]
	if queryDecl.Description != "Custom Query" {
		t.Errorf("YAML config failed to override QueryLens description")
	}

	// 4. Check CreateDigest
	digestDecl := decls[3]
	if digestDecl.Description != "Custom Digest" {
		t.Errorf("YAML config failed to override CreateDigest description")
	}
}
