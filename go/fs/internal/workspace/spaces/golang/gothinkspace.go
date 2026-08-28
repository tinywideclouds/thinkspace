package golang

import (
	"context"
	"fmt"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"google.golang.org/genai"
)

// GoThinkSpace implements ThinkSpace for the Go programming language.
type GoThinkSpace struct {
	config workspace.ThinkSpaceConfig
}

// NewGoThinkSpace injects the externalized YAML configuration.
func NewGoThinkSpace(cfg workspace.ThinkSpaceConfig) *GoThinkSpace {
	return &GoThinkSpace{
		config: cfg,
	}
}

func (s *GoThinkSpace) Name() string {
	return s.config.Name
}

func (s *GoThinkSpace) SystemPrompt() string {
	return s.config.SystemPrompt
}

func (s *GoThinkSpace) SubAgentSystemPrompt() string {
	return s.config.SubAgentSystemPrompt
}

func (s *GoThinkSpace) Model(category workspace.ModelCategory) string {
	return s.config.Models[category]
}

func (s *GoThinkSpace) TurnTimeout() time.Duration {
	return time.Duration(s.config.TurnTimeoutSeconds) * time.Second
}

func (s *GoThinkSpace) AgentTimeout() time.Duration {
	return time.Duration(s.config.AgentTimeoutSeconds) * time.Second
}

func (s *GoThinkSpace) VerifyTimeout() time.Duration {
	return time.Duration(s.config.VerifyTimeoutSeconds) * time.Second
}

func (s *GoThinkSpace) Tools() []*genai.Tool {
	return []*genai.Tool{
		{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Name:        "propose_change",
					Description: s.config.ToolDescription,
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"agent_count": {
								Type:        genai.TypeInteger,
								Description: s.config.AgentCountDescription,
							},
							"agent_instructions": {
								Type:        genai.TypeArray,
								Description: s.config.AgentInstructionsDescription,
								Items: &genai.Schema{
									Type: genai.TypeString,
								},
							},
						},
						Required: []string{"agent_count", "agent_instructions"},
					},
				},
			},
		},
	}
}

func (s *GoThinkSpace) Verify(ctx context.Context, sandbox workspace.CandidateSandbox) error {
	verifyCtx, cancel := context.WithTimeout(ctx, s.VerifyTimeout())
	defer cancel()

	// 1. Black Box Verification: We enforce the contract directly.
	// If the agent didn't write to src/go.mod, it fails.
	if _, err := sandbox.ReadFile(verifyCtx, "src/go.mod"); err != nil {
		return fmt.Errorf("verification failed: no 'go.mod' file found. You must generate a go.mod file in the src/ directory")
	}

	// 2. Black Box Execution: We tell the sandbox WHAT to do, and let it handle the HOW.
	// The Go 1.20 -C flag natively tells the compiler to shift its context to the src/ directory.
	buildOutput, err := sandbox.ExecuteCommand(verifyCtx, "go", "build", "-C", "src", "./...")
	if err != nil {
		if verifyCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("compilation timed out after %s", s.VerifyTimeout())
		}
		return fmt.Errorf("AST syntax or compilation failed: %w\n%s", err, buildOutput)
	}

	testOutput, err := sandbox.ExecuteCommand(verifyCtx, "go", "test", "-C", "src", "./...")
	if err != nil {
		if verifyCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("unit tests timed out (possible infinite loop) after %s", s.VerifyTimeout())
		}
		return fmt.Errorf("unit tests failed: %w\n%s", err, testOutput)
	}

	return nil
}

// func (s *GoThinkSpace) Verify(ctx context.Context, sandbox workspace.CandidateSandbox) error {
// 	verifyCtx, cancel := context.WithTimeout(ctx, s.VerifyTimeout())
// 	defer cancel()

// 	findOutput, err := sandbox.ExecuteCommand(verifyCtx, "find", ".", "-name", "go.mod")
// 	if err != nil || strings.TrimSpace(findOutput) == "" {
// 		return fmt.Errorf("verification failed: no 'go.mod' file found. You must generate a go.mod file in the src/ directory")
// 	}

// 	buildOutput, err := sandbox.ExecuteCommand(verifyCtx, "go", "build", "./...")
// 	if err != nil {
// 		if verifyCtx.Err() == context.DeadlineExceeded {
// 			return fmt.Errorf("compilation timed out after %s", s.VerifyTimeout())
// 		}
// 		return fmt.Errorf("AST syntax or compilation failed: %w\n%s", err, buildOutput)
// 	}

// 	testOutput, err := sandbox.ExecuteCommand(verifyCtx, "go", "test", "./...")
// 	if err != nil {
// 		if verifyCtx.Err() == context.DeadlineExceeded {
// 			return fmt.Errorf("unit tests timed out (possible infinite loop) after %s", s.VerifyTimeout())
// 		}
// 		return fmt.Errorf("unit tests failed: %w\n%s", err, testOutput)
// 	}

// 	return nil
// }
