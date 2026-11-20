package session

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
)

// CandidateProjector can be optionally implemented by a spaces.ThinkState to customize
// how candidate file snapshots are projected for synthesis.
type CandidateProjector interface {
	ProjectCandidate(cand flows.Candidate) string
}

// ProjectCandidateForSpace renders a candidate's changed files using the active ThinkState's
// domain representation (Go AST blueprints for Go spaces, fenced files for prose/generic spaces,
// or unified diff fallback when file snapshots are unavailable).
func ProjectCandidateForSpace(space spaces.ThinkState, cand flows.Candidate) string {
	if custom, ok := space.(CandidateProjector); ok {
		return custom.ProjectCandidate(cand)
	}

	if len(cand.Files) == 0 {
		if strings.TrimSpace(cand.Diff) == "" {
			return "_No file changes recorded._"
		}
		return fmt.Sprintf("<diff>\n%s\n</diff>", strings.TrimSpace(cand.Diff))
	}

	paths := make([]string, 0, len(cand.Files))
	for p := range cand.Files {
		paths = append(paths, p)
	}
	slices.Sort(paths)

	useGoAST := isGoThinkState(space, paths)

	var out strings.Builder
	for i, path := range paths {
		if i > 0 {
			out.WriteString("\n\n")
		}
		content := cand.Files[path]
		if useGoAST && strings.HasSuffix(path, ".go") {
			out.WriteString(projectGoFileAST(path, content))
		} else {
			out.WriteString(projectPlainFile(path, content))
		}
	}

	return out.String()
}

func isGoThinkState(space spaces.ThinkState, paths []string) bool {
	if space != nil {
		if strings.EqualFold(strings.TrimSpace(space.Config().Name), "golang") {
			return true
		}
		if patcher := space.Patcher(); patcher != nil && strings.Contains(fmt.Sprintf("%T", patcher), "GoASTPatcher") {
			return true
		}
	}
	for _, p := range paths {
		if strings.HasSuffix(p, ".go") {
			return true
		}
	}
	return false
}

func projectGoFileAST(path string, content string) string {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return fmt.Sprintf("#### File `%s`\n```go\n%s\n```", path, strings.TrimSpace(content))
	}

	var symbols []string
	var declBlocks []string

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Tok == token.IMPORT {
				continue
			}
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					kind := "type"
					switch s.Type.(type) {
					case *ast.StructType:
						kind = "struct"
					case *ast.InterfaceType:
						kind = "interface"
					}
					symbols = append(symbols, fmt.Sprintf("`%s %s`", kind, s.Name.Name))
				case *ast.ValueSpec:
					for _, name := range s.Names {
						symbols = append(symbols, fmt.Sprintf("`%s %s`", strings.ToLower(d.Tok.String()), name.Name))
					}
				}
			}
			start := fset.Position(d.Pos()).Offset
			end := fset.Position(d.End()).Offset
			if start >= 0 && end <= len(content) && start < end {
				declBlocks = append(declBlocks, strings.TrimSpace(content[start:end]))
			}

		case *ast.FuncDecl:
			sig := formatFuncSignature(content, fset, d)
			symbols = append(symbols, fmt.Sprintf("`%s`", sig))
			start := fset.Position(d.Pos()).Offset
			end := fset.Position(d.End()).Offset
			if start >= 0 && end <= len(content) && start < end {
				declBlocks = append(declBlocks, strings.TrimSpace(content[start:end]))
			}
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "#### File `%s` (`package %s`)\n", path, file.Name.Name)
	if len(symbols) > 0 {
		fmt.Fprintf(&b, "* **AST Symbols:** %s\n", strings.Join(symbols, ", "))
	}
	if len(declBlocks) > 0 {
		fmt.Fprintf(&b, "```go\n%s\n```", strings.Join(declBlocks, "\n\n"))
	} else {
		fmt.Fprintf(&b, "```go\n%s\n```", strings.TrimSpace(content))
	}
	return b.String()
}

func formatFuncSignature(src string, fset *token.FileSet, fn *ast.FuncDecl) string {
	start := fset.Position(fn.Pos()).Offset
	end := fset.Position(fn.Type.End()).Offset
	if start >= 0 && end <= len(src) && start < end {
		sig := strings.Join(strings.Fields(src[start:end]), " ")
		return sig
	}
	return "func " + fn.Name.Name + "()"
}

func projectPlainFile(path string, content string) string {
	lang := "text"
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md":
		lang = "markdown"
	case ".json":
		lang = "json"
	case ".yaml", ".yml":
		lang = "yaml"
	case ".ts":
		lang = "typescript"
	case ".html":
		lang = "html"
	}
	return fmt.Sprintf("#### File `%s`\n```%s\n%s\n```", path, lang, strings.TrimSpace(content))
}

// extractSynthesisReasoning extracts the human-readable merge rationale from the synthesis
// worker's raw response (either the JSON "thought" field or prose preceding the first patch block).
func extractSynthesisReasoning(rawPayload string) string {
	trimmed := strings.TrimSpace(rawPayload)
	if trimmed == "" {
		return ""
	}

	cleaned := trimmed
	if healed, modified := llm.DefaultJSONHealer().Heal(trimmed); modified {
		cleaned = healed
	}

	var parsed struct {
		Thought string `json:"thought"`
	}
	if err := json.Unmarshal([]byte(cleaned), &parsed); err == nil && strings.TrimSpace(parsed.Thought) != "" {
		return strings.TrimSpace(parsed.Thought)
	}

	if idx := strings.Index(trimmed, "### "); idx > 0 {
		return strings.TrimSpace(trimmed[:idx])
	}

	return ""
}
