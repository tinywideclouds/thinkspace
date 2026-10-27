package thinkgraph

import (
	"context"
	"fmt"
	"strings"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
)

type TrajectoryRequest struct {
	Manifest     *ChatManifest
	RecentEvents []Event
	Catalyst     *Event
	Focus        AttentionFocus
	Amendments   map[string]Event // Pre-fetched corrections mapping TargetEventID -> AmendingEvent
}

type TrajectoryBuilder struct{}

func NewTrajectoryBuilder() *TrajectoryBuilder {
	return &TrajectoryBuilder{}
}

func (a *TrajectoryBuilder) Build(ctx context.Context, req TrajectoryRequest) (cognition.Trajectory, error) {
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

	for _, ev := range req.RecentEvents {
		role := "user"
		if ev.Type == EventModel {
			role = "model"
		}
		
		content := ev.Content
		if amendingID, ok := ev.Metadata["amended_by"]; ok && req.Amendments != nil {
			if correction, exists := req.Amendments[amendingID]; exists {
				content = fmt.Sprintf("%s\n\n[System Note: This prompt was later amended to: %s]", content, correction.Content)
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
		
		content := req.Catalyst.Content
		if amendingID, ok := req.Catalyst.Metadata["amended_by"]; ok && req.Amendments != nil {
			if correction, exists := req.Amendments[amendingID]; exists {
				content = fmt.Sprintf("%s\n\n[System Note: This prompt was later amended to: %s]", content, correction.Content)
			}
		}

		trajectory.Catalyst = cognition.TrajectoryEvent{
			Role:    role,
			Content: content,
		}
	}

	return trajectory, nil
}