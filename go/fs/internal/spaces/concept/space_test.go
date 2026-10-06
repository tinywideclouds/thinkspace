package concept_test

import (
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces/concept"
)

func TestConceptSpace_InterfaceContracts(t *testing.T) {
	cfg := spaces.ThinkSpaceConfig{
		Name: "concept",
		Roles: spaces.RolesConfig{
			Manager: spaces.ManagerConfig{
				Tools: spaces.ToolsConfig{
					ProposeChange: spaces.ToolProposeChangeConfig{Description: "propose"},
					QueryLens:     spaces.ToolQueryLensConfig{Description: "query"},
					CreateDigest:  spaces.ToolCreateDigestConfig{Description: "digest"},
				},
			},
		},
	}

	space := concept.NewConceptSpace(cfg)

	if space.Config().Name != "concept" {
		t.Errorf("Config() failed to return injected config")
	}
	if space.Verifier() == nil {
		t.Errorf("Verifier() returned nil")
	}
	if space.Mapbook() == nil {
		t.Errorf("Mapbook() returned nil")
	}
	if space.Patcher() == nil {
		t.Errorf("Patcher() returned nil")
	}

	tools := space.Tools()
	if len(tools) != 3 {
		t.Fatalf("Expected exactly 3 tools, got %d", len(tools))
	}

	toolMap := make(map[string]bool)
	for _, tool := range tools {
		toolMap[tool.Name] = true
	}

	expectedTools := []string{"propose_change", "query_lens", "create_digest"}
	for _, expected := range expectedTools {
		if !toolMap[expected] {
			t.Errorf("Missing expected tool: %s", expected)
		}
	}
}
