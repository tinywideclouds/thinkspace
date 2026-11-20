package spaces

import (
	"context"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// Verifier defines the contract for evaluating a candidate workspace's code.
type Verifier interface {
	Verify(ctx context.Context, runner workspace.CommandRunner) error
}
