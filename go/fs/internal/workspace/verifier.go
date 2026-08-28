package workspace

import "context"

// Verifier defines the contract for evaluating a candidate sandbox's code.
type Verifier interface {
	Verify(ctx context.Context, sandbox CandidateSandbox) error
}
