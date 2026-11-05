package concept_test

import (
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces/concept"
)

func TestConceptState_InterfaceContracts(t *testing.T) {
	cfg := spaces.ThinkStateConfig{
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

	space := concept.NewConceptState(cfg)

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

}
