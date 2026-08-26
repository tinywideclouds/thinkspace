package golang

import (
	"bytes"
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func (s *GoThinkSpace) Verify(ctx context.Context, dir string) error {
	fset := token.NewFileSet()
	astErrorFound := false
	var astErrorMsg strings.Builder
	var goModDir string

	// 1. Walk the directory to parse AST and locate go.mod
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			pkgs, err := parser.ParseDir(fset, path, nil, parser.AllErrors)
			if err != nil {
				astErrorFound = true
				astErrorMsg.WriteString(fmt.Sprintf("Syntax error in %s: %v\n", path, err))
			}
			for _, pkg := range pkgs {
				_ = pkg
			}
		} else if info.Name() == "go.mod" {
			goModDir = filepath.Dir(path)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory for AST parsing: %w", err)
	}

	if astErrorFound {
		return fmt.Errorf("AST syntax verification failed:\n%s", astErrorMsg.String())
	}

	// 2. Enforce the go.mod requirement
	if goModDir == "" {
		return fmt.Errorf("verification failed: no 'go.mod' file found. You must generate a go.mod file in the src/ directory")
	}

	verifyCtx, cancel := context.WithTimeout(ctx, s.VerifyTimeout())
	defer cancel()

	// 3. Run 'go build' from the directory containing go.mod
	buildCmd := exec.CommandContext(verifyCtx, "go", "build", "./...")
	buildCmd.Dir = goModDir

	var buildStdout, buildStderr bytes.Buffer
	buildCmd.Stdout = &buildStdout
	buildCmd.Stderr = &buildStderr

	if err := buildCmd.Run(); err != nil {
		if verifyCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("compilation timed out after %s", s.VerifyTimeout())
		}
		return fmt.Errorf("compilation failed: %w\n%s", err, buildStderr.String())
	}

	// 4. Run 'go test' to ensure the requested unit tests actually pass
	testCmd := exec.CommandContext(verifyCtx, "go", "test", "./...")
	testCmd.Dir = goModDir

	var testStdout, testStderr bytes.Buffer
	testCmd.Stdout = &testStdout
	testCmd.Stderr = &testStderr

	if err := testCmd.Run(); err != nil {
		if verifyCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("unit tests timed out (possible infinite loop) after %s", s.VerifyTimeout())
		}
		return fmt.Errorf("unit tests failed: %w\n%s\n%s", err, testStderr.String(), testStdout.String())
	}

	return nil
}
