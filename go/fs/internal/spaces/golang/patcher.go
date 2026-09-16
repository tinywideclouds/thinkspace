package golang

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"path/filepath"

	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type GoASTPatcher struct {
	instructions string
}

func NewGoASTPatcher(instructions string) *GoASTPatcher {
	return &GoASTPatcher{
		instructions: instructions,
	}
}

func (p *GoASTPatcher) SystemInstructions() string {
	return p.instructions
}

type PatchRequest struct {
	File   string `json:"file"`
	Action string `json:"action"`
	Name   string `json:"name,omitempty"`
	Code   string `json:"code"`
}

func (p *GoASTPatcher) Apply(ctx context.Context, sandbox workspace.CandidateSandbox, llmOutput string) error {
	var payload struct {
		Patches []PatchRequest `json:"patches"`
	}

	if err := json.Unmarshal([]byte(llmOutput), &payload); err != nil {
		return fmt.Errorf("invalid patch JSON: %w", err)
	}

	for _, patch := range payload.Patches {
		cleanPath := filepath.ToSlash(filepath.Clean(patch.File))

		args := []any{"action", patch.Action, "file", cleanPath}
		if patch.Name != "" {
			args = append(args, "name", patch.Name)
		}
		slog.InfoContext(ctx, "applied AST patch", args...)

		if patch.Action == "full_replace" {
			if err := sandbox.WriteFile(ctx, cleanPath, []byte(patch.Code)); err != nil {
				return err
			}
			continue
		}

		src, err := sandbox.ReadFile(ctx, cleanPath)
		if err != nil {
			return fmt.Errorf("cannot patch missing file %s. Use full_replace to create new files", cleanPath)
		}

		if patch.Action == "add_code" {
			src = append(src, []byte("\n\n"+patch.Code)...)
			if err := sandbox.WriteFile(ctx, cleanPath, src); err != nil {
				return err
			}
			continue
		}

		if patch.Action == "replace_function" {
			newSrc, err := p.replaceFunctionAST(src, patch.Name, patch.Code)
			if err != nil {
				return fmt.Errorf("failed to apply AST patch to %s: %w", cleanPath, err)
			}
			if err := sandbox.WriteFile(ctx, cleanPath, newSrc); err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *GoASTPatcher) replaceFunctionAST(src []byte, funcName string, newCode string) ([]byte, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("existing code has invalid syntax: %w", err)
	}

	var startOffset, endOffset int
	found := false

	ast.Inspect(node, func(n ast.Node) bool {
		if decl, ok := n.(*ast.FuncDecl); ok {
			if decl.Name.Name == funcName {
				startOffset = fset.Position(decl.Pos()).Offset
				endOffset = fset.Position(decl.End()).Offset
				found = true
				return false
			}
		}
		return true
	})

	if !found {
		return nil, fmt.Errorf("function '%s' not found in file", funcName)
	}

	var newSrc []byte
	newSrc = append(newSrc, src[:startOffset]...)
	newSrc = append(newSrc, []byte(newCode)...)
	newSrc = append(newSrc, src[endOffset:]...)

	return newSrc, nil
}
