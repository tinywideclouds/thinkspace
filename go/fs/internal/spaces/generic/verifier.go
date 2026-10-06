package generic

import (
	"context"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type NoOpVerifier struct{}

func NewNoOpVerifier() *NoOpVerifier {
	return &NoOpVerifier{}
}

func (v *NoOpVerifier) Verify(ctx context.Context, sandbox workspace.CandidateSandbox) error {
	// Prose does not compile. Instantly pass verification.
	return nil
}
