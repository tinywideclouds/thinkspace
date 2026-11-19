package thinkgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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
	switch string(node.Type) {
	case "flow_lifecycle":
		return fmt.Sprintf("[System: A sub-agent flow is executing or completed. Result: %s]", node.Content)
	case "candidate_resolution":
		return fmt.Sprintf("[System: The user resolved a proposal. Result: %s]", node.Content)
	case string(EventPrompt):
		if len(node.Data) > 0 {
			var p Prompt
			if err := json.Unmarshal(node.Data, &p); err == nil && p.Content != "" {
				return p.Content
			}
		}
	case string(EventModel):
		if len(node.Data) > 0 {
			var m ModelResponse
			if err := json.Unmarshal(node.Data, &m); err == nil && m.Content != "" {
				return m.Content
			}
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

	if req.Manifest != nil {
		contexts := make([]SpaceContext, 0, len(req.Manifest.SpaceContexts))
		for _, sc := range req.Manifest.SpaceContexts {
			contexts = append(contexts, sc)
		}
		sort.Slice(contexts, func(i, j int) bool {
			return contexts[i].ID.String() < contexts[j].ID.String()
		})
		for _, sc := range contexts {
			trajectory.LongTermMemory = append(trajectory.LongTermMemory, sc.Content)
		}

		digests := make([]DigestMeta, 0, len(req.Manifest.Digests))
		for _, d := range req.Manifest.Digests {
			digests = append(digests, d)
		}
		sort.Slice(digests, func(i, j int) bool {
			return digests[i].ID.String() < digests[j].ID.String()
		})
		for _, d := range digests {
			trajectory.LongTermMemory = append(trajectory.LongTermMemory, d.Summary)
		}
	}

	for _, node := range req.RecentEvents {
		if node.Type == EventSystem {
			continue
		}

		role := "user"
		if node.Type == EventModel {
			role = "model"
		}

		content := a.extractContent(node)
		if content == "" {
			continue
		}

		if amendingID, ok := node.Metadata["amended_by"]; ok && req.Amendments != nil {
			if correctionNode, exists := req.Amendments[amendingID]; exists {
				correctionContent := a.extractContent(correctionNode)
				content = fmt.Sprintf("%s\n\n[System Note: This prompt was later amended to: %s]", content, correctionContent)
			}
		}

		if len(trajectory.WorkingMemory) > 0 && trajectory.WorkingMemory[len(trajectory.WorkingMemory)-1].Role == role {
			trajectory.WorkingMemory[len(trajectory.WorkingMemory)-1].Content += "\n\n" + content
		} else {
			trajectory.WorkingMemory = append(trajectory.WorkingMemory, cognition.TrajectoryEvent{
				Role:    role,
				Content: content,
			})
		}
	}

	if req.Catalyst == nil {
		return cognition.Trajectory{}, fmt.Errorf("invalid trajectory state: catalyst node is nil")
	}

	role := "user"
	if req.Catalyst.Type == EventModel {
		role = "model"
	}

	content := a.extractContent(*req.Catalyst)
	if content == "" {
		return cognition.Trajectory{}, fmt.Errorf("invalid trajectory state: catalyst node %s has empty content", req.Catalyst.ID)
	}

	if amendingID, ok := req.Catalyst.Metadata["amended_by"]; ok && req.Amendments != nil {
		if correctionNode, exists := req.Amendments[amendingID]; exists {
			correctionContent := a.extractContent(correctionNode)
			content = fmt.Sprintf("%s\n\n[System Note: This prompt was later amended to: %s]", content, correctionContent)
		}
	}

	if len(trajectory.WorkingMemory) > 0 && trajectory.WorkingMemory[len(trajectory.WorkingMemory)-1].Role == role {
		trailing := trajectory.WorkingMemory[len(trajectory.WorkingMemory)-1]
		trajectory.WorkingMemory = trajectory.WorkingMemory[:len(trajectory.WorkingMemory)-1]
		content = trailing.Content + "\n\n" + content
	}

	trajectory.Catalyst = cognition.TrajectoryEvent{
		Role:    role,
		Content: content,
	}

	return trajectory, nil
}
