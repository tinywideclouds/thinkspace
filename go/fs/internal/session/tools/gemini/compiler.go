package gemini

import (
	"google.golang.org/genai"

	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
)

func CompileCoreTools(
	transition *tools.TransitionThinkStateTool,
	propose *tools.ProposeChangeTool,
	query *tools.QueryLensTool,
	digest *tools.CreateDigestTool,
	config spaces.ThinkStateConfig,
) []*genai.Tool {
	var declarations []*genai.FunctionDeclaration

	if transition != nil {
		declarations = append(declarations, CompileTransitionThinkState(transition))
	}
	if propose != nil {
		declarations = append(declarations, CompileProposeChange(propose, config))
	}
	if query != nil {
		declarations = append(declarations, CompileQueryLens(query, config))
	}
	if digest != nil {
		declarations = append(declarations, CompileCreateDigest(digest, config))
	}

	if len(declarations) == 0 {
		return nil
	}

	return []*genai.Tool{{FunctionDeclarations: declarations}}
}
