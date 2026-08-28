package golang

import (
	"context"
	"fmt"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type GoVerifier struct {
	timeout time.Duration
}

func NewGoVerifier(timeout time.Duration) *GoVerifier {
	return &GoVerifier{timeout: timeout}
}

func (g *GoVerifier) Verify(ctx context.Context, sandbox workspace.CandidateSandbox) error {
	verifyCtx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	// 1. Black Box Verification
	if _, err := sandbox.ReadFile(verifyCtx, "src/go.mod"); err != nil {
		return fmt.Errorf("verification failed: no 'go.mod' file found. You must generate a go.mod file in the src/ directory")
	}

	// 2. Black Box Execution (Build)
	buildOutput, err := sandbox.ExecuteCommand(verifyCtx, "go", "build", "-C", "src", "./...")
	if err != nil {
		if verifyCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("compilation timed out after %s", g.timeout)
		}
		return fmt.Errorf("AST syntax or compilation failed: %w\n%s", err, buildOutput)
	}

	// 3. Black Box Execution (Test)
	testOutput, err := sandbox.ExecuteCommand(verifyCtx, "go", "test", "-C", "src", "./...")
	if err != nil {
		if verifyCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("unit tests timed out (possible infinite loop) after %s", g.timeout)
		}
		return fmt.Errorf("unit tests failed: %w\n%s", err, testOutput)
	}

	return nil
}
