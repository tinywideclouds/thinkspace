package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

const QueryLens ToolName = "query_lens"
const Tag = "tag"

type QueryLensRequest struct {
	Tag       string `json:"tag"`
	Reasoning string `json:"reasoning"`
}

type QueryLensTool struct {
	workspace Workspace
}

func NewQueryLensTool(workspace Workspace) *QueryLensTool {
	return &QueryLensTool{workspace: workspace}
}

func (t *QueryLensTool) Name() ToolName {
	return QueryLens
}

func (t *QueryLensTool) DefaultDescription() string {
	return "Query historical events indexed under a specific semantic tag."
}

func (t *QueryLensTool) Execute(ctx context.Context, executionContext ExecutionContext) (bool, error) {
	var request QueryLensRequest
	argumentBytes, err := json.Marshal(executionContext.RawArguments)
	if err != nil {
		return false, fmt.Errorf("marshaling query_lens arguments: %w", err)
	}
	if err := json.Unmarshal(argumentBytes, &request); err != nil {
		return false, fmt.Errorf("invalid query_lens schema: %w", err)
	}

	executionContext.UI.OnTextChunk(fmt.Sprintf("\n\n🔍 **Searching memory for #%s...**\n", request.Tag))

	var toolResponse strings.Builder
	fmt.Fprintf(&toolResponse, "[System Tool Response: query_lens(%s)]\n", request.Tag)

	chatObj := &thinkgraph.Chat{ID: executionContext.ChatID, SpaceID: executionContext.SpaceID}
	graph := t.workspace.Graph()

	if graph == nil {
		toolResponse.WriteString("Error: Graph database is not initialized.\n")
	} else {
		lensResult, err := graph.GetLenses(ctx, chatObj, request.Tag)
		if err != nil {
			toolResponse.WriteString(fmt.Sprintf("Error fetching events: %v\n", err))
		} else if len(lensResult.Nodes) == 0 && len(lensResult.Digests) == 0 {
			toolResponse.WriteString("No historical events found for this tag.\n")
		} else {
			for _, node := range lensResult.Nodes {
				fmt.Fprintf(&toolResponse, "- [%s] %s: %s\n", node.Timestamp.Format("15:04:05"), node.Type, node.Content)
			}
		}
	}

	executionContext.Bundle.Trajectory.WorkingMemory = append(executionContext.Bundle.Trajectory.WorkingMemory, cognition.TrajectoryEvent{
		Role:    "user",
		Content: toolResponse.String(),
	})

	return false, nil
}
