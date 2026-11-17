package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
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

func (t *QueryLensTool) Execute(ctx context.Context, executionContext ExecutionContext) (ToolResponse, error) {
	var request QueryLensRequest
	argumentBytes, err := json.Marshal(executionContext.RawArguments)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("marshaling query_lens arguments: %w", err)
	}
	if err := json.Unmarshal(argumentBytes, &request); err != nil {
		return ToolResponse{}, fmt.Errorf("invalid query_lens schema: %w", err)
	}

	request.Tag = strings.TrimSpace(request.Tag)
	if request.Tag == "" {
		return ToolResponse{}, fmt.Errorf("query_lens tag cannot be empty")
	}

	if executionContext.Emitter != nil {
		executionContext.Emitter.Emit("manager", telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "tool_status",
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("Searching memory for #%s...", request.Tag),
		})
	}

	var toolResponse strings.Builder
	fmt.Fprintf(&toolResponse, "[System Tool Response: query_lens(%s)]\n", request.Tag)

	if t.workspace == nil || t.workspace.Graph() == nil {
		toolResponse.WriteString("Error: Graph database is not initialized.\n")
	} else {
		lensResult, getErr := t.workspace.Graph().GetLenses(ctx, executionContext.Address, request.Tag)
		if getErr != nil {
			toolResponse.WriteString(fmt.Sprintf("Error fetching events: %v\n", getErr))
		} else if len(lensResult.Nodes) == 0 && len(lensResult.Digests) == 0 && len(lensResult.SpaceContexts) == 0 {
			toolResponse.WriteString("No historical events found for this tag.\n")
		} else {
			for _, sc := range lensResult.SpaceContexts {
				fmt.Fprintf(&toolResponse, "- [Rule] %s\n", sc.Content)
			}
			for _, digest := range lensResult.Digests {
				fmt.Fprintf(&toolResponse, "- [Digest] %s\n", digest.Summary)
			}
			for _, node := range lensResult.Nodes {
				fmt.Fprintf(&toolResponse, "- [%s] %s: %s\n", node.Timestamp.Format("15:04:05"), node.Type, node.Content)
			}
		}
	}

	if executionContext.Bundle != nil {
		executionContext.Bundle.Trajectory.WorkingMemory = append(executionContext.Bundle.Trajectory.WorkingMemory, cognition.TrajectoryEvent{
			Role:    "user",
			Content: toolResponse.String(),
		})
	}

	return ToolResponse{}, nil
}
