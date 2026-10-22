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
}

type TrajectoryBuilder struct{}

func NewTrajectoryBuilder() *TrajectoryBuilder {
	return &TrajectoryBuilder{}
}

func (a *TrajectoryBuilder) Build(ctx context.Context, req TrajectoryRequest) (cognition.Trajectory, error) {
	var traj cognition.Trajectory

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

	traj.LongTermMemory = append(traj.LongTermMemory, metadata.String())

	for _, sc := range req.Manifest.SpaceContexts {
		traj.LongTermMemory = append(traj.LongTermMemory, sc.Content)
	}

	for _, d := range req.Manifest.Digests {
		if d.Promoted {
			traj.LongTermMemory = append(traj.LongTermMemory, d.Summary)
		}
	}

	for _, ev := range req.RecentEvents {
		role := "user"
		if ev.Type == EventModel {
			role = "model"
		}
		traj.WorkingMemory = append(traj.WorkingMemory, cognition.TrajectoryEvent{
			Role:    role,
			Content: ev.Content,
		})
	}

	if req.Catalyst != nil {
		role := "user"
		if req.Catalyst.Type == EventModel {
			role = "model"
		}
		traj.Catalyst = cognition.TrajectoryEvent{
			Role:    role,
			Content: req.Catalyst.Content,
		}
	}

	return traj, nil
}
