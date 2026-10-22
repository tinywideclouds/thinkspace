package thinkgraph

import (
	"context"
	"fmt"
	"strings"
	"uuid"
)

// GraphSubscriber mirrors all events and digests to the BadgerDB fast layer.
type GraphSubscriber struct {
	graph GraphRepository
}

func NewGraphSubscriber(graph GraphRepository) *GraphSubscriber {
	return &GraphSubscriber{graph: graph}
}

func (g *GraphSubscriber) OnEvent(ctx context.Context, c *Chat, event Event) error {
	if event.Type == EventDigest {
		isPromoted := false
		if val, ok := event.Metadata["is_sticky"]; ok && val == "true" {
			isPromoted = true
		}

		var summarized []uuid.UUID
		if val, ok := event.Metadata["summarizes"]; ok && val != "" {
			for _, idStr := range strings.Split(val, ",") {
				if id, err := uuid.Parse(strings.TrimSpace(idStr)); err == nil {
					summarized = append(summarized, id)
				}
			}
		}

		digestMeta := DigestMeta{
			ID:               event.ID,
			Summary:          event.Content,
			Promoted:         isPromoted,
			Tags:             event.Tags,
			SummarizedEvents: summarized,
		}

		if err := g.graph.SaveDigest(ctx, c, digestMeta); err != nil {
			return fmt.Errorf("failed to save digest to graph: %w", err)
		}
		return nil
	}

	if err := g.graph.AppendEvent(ctx, c, event); err != nil {
		return fmt.Errorf("failed to append event to graph: %w", err)
	}

	// Safely increment the Graph's total events state tracking
	state, err := g.graph.GetSyncState(ctx, c)
	if err != nil {
		return fmt.Errorf("failed to read sync state: %w", err)
	}
	if state == nil {
		state = &LedgerManifest{TotalEvents: 0}
	}

	state.TotalEvents++
	if err := g.graph.SetSyncState(ctx, c, *state); err != nil {
		return fmt.Errorf("failed to update sync state: %w", err)
	}

	return nil
}
