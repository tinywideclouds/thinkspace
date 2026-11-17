package generic

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// CIVerifier executes a YAML-defined list of commands inside the candidate sandbox.
type CIVerifier struct {
	toolchain spaces.ToolchainConfig
	timeout   time.Duration
}

func NewCIVerifier(toolchain spaces.ToolchainConfig, timeout time.Duration) *CIVerifier {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &CIVerifier{
		toolchain: toolchain,
		timeout:   timeout,
	}
}

func (v *CIVerifier) Verify(ctx context.Context, sandbox workspace.CandidateSandbox) error {
	verifyCtx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	var trace strings.Builder
	hasError := false

	runPhase := func(phaseName string, commands []string, strict bool) {
		for _, cmdStr := range commands {
			if verifyCtx.Err() != nil {
				trace.WriteString(fmt.Sprintf("[%s] %s: TIMEOUT\n", phaseName, cmdStr))
				if strict {
					hasError = true
				}
				break
			}

			parts := strings.Fields(cmdStr)
			if len(parts) == 0 {
				continue
			}

			cmd := parts[0]
			args := parts[1:]

			output, err := sandbox.ExecuteCommand(verifyCtx, cmd, args...)
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) || errors.Is(verifyCtx.Err(), context.DeadlineExceeded) {
					trace.WriteString(fmt.Sprintf("[%s] %s: TIMEOUT\n%s\n\n", phaseName, cmdStr, output))
					if strict {
						hasError = true
					}
					break
				}

				trace.WriteString(fmt.Sprintf("[%s] %s: FAILED\n%s\n%v\n\n", phaseName, cmdStr, output, err))

				if strict {
					hasError = true
					break
				}
			} else if output != "" {
				trace.WriteString(fmt.Sprintf("[%s] %s: OK\n%s\n\n", phaseName, cmdStr, output))
			}
		}
	}

	// Phase 1: Best-effort auto-fixing (Non-Strict)
	runPhase("PRE-VERIFY", v.toolchain.PreVerify, false)

	// Phase 2: Actual Verification (Strict)
	if verifyCtx.Err() == nil {
		runPhase("VERIFY", v.toolchain.Verify, true)
	} else {
		hasError = true
	}

	if hasError {
		return fmt.Errorf("verification failed:\n%s", strings.TrimSpace(trace.String()))
	}

	return nil
}
