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
		// Strict LLM context isolation: Omit internal system noise entirely
		if node.Type == EventSystem {
			continue
		}

		role := "user"
		if node.Type == EventModel {
			role = "model"
		}

		content := a.extractContent(node)

		// If the node represents an empty placeholder (like an interrupted tool call shell), skip appending it
		if strings.TrimSpace(content) == "" {
			continue
		}

		if amendingID, ok := node.Metadata["amended_by"]; ok && req.Amendments != nil {
			if correctionNode, exists := req.Amendments[amendingID]; exists {
				correctionContent := a.extractContent(correctionNode)
				content = fmt.Sprintf("%s\n\n[System Note: This prompt was later amended to: %s]", content, correctionContent)
			}
		}

		// Role Collapsing: merge contiguous roles to prevent LLM API turn-taking violations
		if len(trajectory.WorkingMemory) > 0 && trajectory.WorkingMemory[len(trajectory.WorkingMemory)-1].Role == role {
			trajectory.WorkingMemory[len(trajectory.WorkingMemory)-1].Content += "\n\n" + content
		} else {
			trajectory.WorkingMemory = append(trajectory.WorkingMemory, cognition.TrajectoryEvent{
				Role:    role,
				Content: content,
			})
		}
	}

	if req.Catalyst != nil && req.Catalyst.Type != EventSystem {
		role := "user"
		if req.Catalyst.Type == EventModel {
			role = "model"
		}

		content := a.extractContent(*req.Catalyst)
		if strings.TrimSpace(content) != "" {
			if amendingID, ok := req.Catalyst.Metadata["amended_by"]; ok && req.Amendments != nil {
				if correctionNode, exists := req.Amendments[amendingID]; exists {
					correctionContent := a.extractContent(correctionNode)
					content = fmt.Sprintf("%s\n\n[System Note: This prompt was later amended to: %s]", content, correctionContent)
				}
			}

			// If Catalyst is the same role as the last item in memory, collapse it into WorkingMemory instead
			if len(trajectory.WorkingMemory) > 0 && trajectory.WorkingMemory[len(trajectory.WorkingMemory)-1].Role == role {
				trajectory.WorkingMemory[len(trajectory.WorkingMemory)-1].Content += "\n\n" + content
			} else {
				trajectory.Catalyst = cognition.TrajectoryEvent{
					Role:    role,
					Content: content,
				}
			}
		}
	}

	// Defensive Integrity Check: Ensure we don't send a trajectory ending on a model turn,
	// which will crash the Gemini API. If the final working memory node is a model,
	// it should only happen if the Catalyst was empty (e.g. recovering state).
	if trajectory.Catalyst.Content == "" && len(trajectory.WorkingMemory) > 0 {
		lastIdx := len(trajectory.WorkingMemory) - 1
		if trajectory.WorkingMemory[lastIdx].Role == "model" {
			return cognition.Trajectory{}, fmt.Errorf("invalid trajectory state: working memory ends on a model turn and lacks a catalyst")
		}
	}

	return trajectory, nil
}
