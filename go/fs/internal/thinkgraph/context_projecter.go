package thinkgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
)

type ContextRequest struct {
	Manifest     *ChatManifest
	RecentEvents []Node
	Catalyst     *Node
	Focus        AttentionFocus
	Amendments   map[string]Node
}

type ContextProjecter struct{}

func NewContextProjecter() *ContextProjecter {
	return &ContextProjecter{}
}

func (a *ContextProjecter) extractContent(node Node) string {
	if len(node.Data) > 0 {
		switch string(node.Type) {
		case string(EventPrompt):
			var p Prompt
			if err := json.Unmarshal(node.Data, &p); err == nil && p.Content != "" {
				return p.Content
			}
		case string(EventModel):
			var m ModelResponse
			if err := json.Unmarshal(node.Data, &m); err == nil && m.Content != "" {
				return m.Content
			}
		// Safely ignore MaterializedState diffs. Just extract semantic summary.
		case "flow_lifecycle":
			return fmt.Sprintf("[System: A sub-agent flow is executing or completed. Result: %s]", node.Content)
		case "candidate_resolution":
			return fmt.Sprintf("[System: The user resolved a proposal. Result: %s]", node.Content)
		}
	}
	return node.Content
}

func (a *ContextProjecter) Build(ctx context.Context, req ContextRequest) (cognition.Trajectory, error) {
	var trajectory cognition.Trajectory

	var metadata strings.Builder
	metadata.WriteString("[CONTEXT HORIZON METADATA]\n")

	if len(req.Focus.PrimaryFocus) > 0 {
		metadata.WriteString(fmt.Sprintf("Active Topic Focus: %s\n", strings.Join(req.Focus.PrimaryFocus, ", ")))
	}
	if len(req.Focus.RelatedContext) > 0 {
		metadata.WriteString(fmt.Sprintf("Active Tag Bridges: %s\n", strings.Join(req.Focus.RelatedContext, ", ")))
	}
	if len(req.Focus.DormantLenses) > 0 {
		metadata.WriteString(fmt.Sprintf("Dormant Memories Available via query_lens: %s\n", strings.Join(req.Focus.DormantLenses, ", ")))
	}

	trajectory.LongTermMemory = append(trajectory.LongTermMemory, metadata.String())

	for _, sc := range req.Manifest.SpaceContexts {
		trajectory.LongTermMemory = append(trajectory.LongTermMemory, sc.Content)
	}

	for _, d := range req.Manifest.Digests {
		if d.Promoted {
			trajectory.LongTermMemory = append(trajectory.LongTermMemory, d.Summary)
		}
	}

	for _, node := range req.RecentEvents {
		role := "user"
		if node.Type == EventModel {
			role = "model"
		}

		content := a.extractContent(node)
		if amendingID, ok := node.Metadata["amended_by"]; ok && req.Amendments != nil {
			if correctionNode, exists := req.Amendments[amendingID]; exists {
				correctionContent := a.extractContent(correctionNode)
				content = fmt.Sprintf("%s\n\n[System Note: This prompt was later amended to: %s]", content, correctionContent)
			}
		}

		trajectory.WorkingMemory = append(trajectory.WorkingMemory, cognition.TrajectoryEvent{
			Role:    role,
			Content: content,
		})
	}

	if req.Catalyst != nil {
		role := "user"
		if req.Catalyst.Type == EventModel {
			role = "model"
		}

		content := a.extractContent(*req.Catalyst)
		if amendingID, ok := req.Catalyst.Metadata["amended_by"]; ok && req.Amendments != nil {
			if correctionNode, exists := req.Amendments[amendingID]; exists {
				correctionContent := a.extractContent(correctionNode)
				content = fmt.Sprintf("%s\n\n[System Note: This prompt was later amended to: %s]", content, correctionContent)
			}
		}

		trajectory.Catalyst = cognition.TrajectoryEvent{
			Role:    role,
			Content: content,
		}
	}

	return trajectory, nil
}
