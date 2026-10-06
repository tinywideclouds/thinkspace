package generic_test

import (
	"context"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/spaces/generic"
)

func TestNoOpVerifier_Verify(t *testing.T) {
	verifier := generic.NewNoOpVerifier()
	// Should immediately return nil without attempting to access the sandbox
	err := verifier.Verify(context.Background(), nil)
	if err != nil {
		t.Errorf("Expected NoOpVerifier to return nil, got %v", err)
	}
}
