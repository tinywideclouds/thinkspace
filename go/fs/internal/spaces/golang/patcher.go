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
	"slices"
	"strings"

	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"golang.org/x/tools/imports"
)

type GoASTPatcher struct {
	instructions  string
	formatImports bool
}

func NewGoASTPatcher(instructions string, formatImports bool) *GoASTPatcher {
	return &GoASTPatcher{
		instructions:  instructions,
		formatImports: formatImports,
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

func (p *GoASTPatcher) Apply(ctx context.Context, fs workspace.FileWorkspace, llmOutput string) error {
	cleanedOutput := llmOutput
	if healed, modified := llm.DefaultJSONHealer().Heal(llmOutput); modified {
		cleanedOutput = healed
	}

	var payload struct {
		Patches []PatchRequest `json:"patches"`
	}

	if err := json.Unmarshal([]byte(cleanedOutput), &payload); err != nil {
		return fmt.Errorf("invalid patch JSON: %w", err)
	}

	for _, patch := range payload.Patches {
		rawPath := strings.TrimSpace(patch.File)
		if rawPath == "" {
			return fmt.Errorf("patch entry is missing target file path")
		}

		cleanPath := filepath.ToSlash(filepath.Clean(rawPath))
		if filepath.IsAbs(cleanPath) || cleanPath == "." || cleanPath == ".." || strings.HasPrefix(cleanPath, "../") {
			return fmt.Errorf("invalid or escaping target file path: %q", patch.File)
		}

		args := []any{"action", patch.Action, "file", cleanPath}
		if patch.Name != "" {
			args = append(args, "name", patch.Name)
		}
		slog.InfoContext(ctx, "applied AST patch", args...)

		switch patch.Action {
		case "full_replace":
			if err := p.writeFile(ctx, fs, cleanPath, []byte(patch.Code)); err != nil {
				return err
			}

		case "add_code":
			src, err := fs.ReadFile(ctx, cleanPath)
			if err != nil {
				return fmt.Errorf("cannot patch missing file %s (use full_replace to create new files): %w", cleanPath, err)
			}
			combined := append(slices.Clone(src), []byte("\n\n"+patch.Code)...)
			if err := p.writeFile(ctx, fs, cleanPath, combined); err != nil {
				return err
			}

		case "replace_function":
			src, err := fs.ReadFile(ctx, cleanPath)
			if err != nil {
				return fmt.Errorf("cannot patch missing file %s (use full_replace to create new files): %w", cleanPath, err)
			}
			newSrc, err := p.replaceFunctionAST(src, patch.Name, patch.Code)
			if err != nil {
				return fmt.Errorf("failed to apply AST patch to %s: %w", cleanPath, err)
			}
			if err := p.writeFile(ctx, fs, cleanPath, newSrc); err != nil {
				return err
			}

		default:
			return fmt.Errorf("unsupported patch action %q for file %s", patch.Action, cleanPath)
		}
	}

	return nil
}

func (p *GoASTPatcher) writeFile(ctx context.Context, fs workspace.FileWorkspace, cleanPath string, data []byte) error {
	out := data
	if p.formatImports && strings.HasSuffix(cleanPath, ".go") {
		targetPath := cleanPath
		if resolved := fs.ResolvePath(cleanPath); resolved != "" {
			targetPath = resolved
		}

		formatted, err := imports.Process(targetPath, data, nil)
		if err != nil {
			slog.WarnContext(ctx, "goimports formatting skipped due to parse error; writing raw source", "file", cleanPath, "error", err)
		} else {
			out = formatted
		}
	}

	return fs.WriteFile(ctx, cleanPath, out)
}

func (p *GoASTPatcher) replaceFunctionAST(src []byte, funcName string, newCode string) ([]byte, error) {
	if strings.TrimSpace(funcName) == "" {
		return nil, fmt.Errorf("replace_function requires a non-empty function name")
	}

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
