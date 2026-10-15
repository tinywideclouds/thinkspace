package thinkgraph

import (
	"context"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
)

type AssemblyRequest struct {
	Manifest     *ChatManifest
	RecentEvents []Event
	Catalyst     *Event // The explicit triggering event (usually a user prompt) that initiated the current assembly request.
}

type Assembler struct{}

func NewAssembler() *Assembler {
	return &Assembler{}
}

func (a *Assembler) Build(ctx context.Context, req AssemblyRequest) (cognition.Trajectory, error) {
	var traj cognition.Trajectory

	for _, d := range req.Manifest.Digests {
		if d.IsSticky {
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
