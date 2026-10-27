package thinkgraph

import (
	"context"
)

// TraversalEngine safely stitches timelines across branches by consulting the graph topology.
type TraversalEngine struct {
	graph GraphRepository
}

func NewTraversalEngine(graph GraphRepository) *TraversalEngine {
	return &TraversalEngine{graph: graph}
}

func (t *TraversalEngine) GetTimelinePage(
	ctx context.Context,
	c *Chat,
	cursor string,
	limit int,
	topology map[string]BranchTopology,
) ([]Event, string, bool, error) {

	var finalEvents []Event
	currentBranch := c.ActiveBranch
	if currentBranch == "" {
		currentBranch = "main"
	}
	currentCursor := cursor
	remainingLimit := limit

	for remainingLimit > 0 {
		events, nextCursor, hasMore, err := t.graph.GetBranchTimelinePage(ctx, c, currentBranch, currentCursor, remainingLimit)
		if err != nil {
			return nil, "", false, err
		}

		finalEvents = append(events, finalEvents...) // Prepend since Badger returns oldest-first within the limit block
		remainingLimit -= len(events)

		if hasMore {
			return finalEvents, nextCursor, true, nil
		}

		// Reached the end of the current branch. Hop to the parent if it exists.
		topo, exists := topology[currentBranch]
		if !exists || topo.ParentBranch == "" {
			break // Root of the entire chat
		}

		currentBranch = topo.ParentBranch
		currentCursor = topo.ForkPoint
	}

	hasMoreTotal := false
	var finalCursor string
	if remainingLimit == 0 {
		hasMoreTotal = true
		if len(finalEvents) > 0 {
			finalCursor = finalEvents[0].ID.String()
		}
	}

	return finalEvents, finalCursor, hasMoreTotal, nil
}

func (t *TraversalEngine) WalkTimelineReverse(
	ctx context.Context,
	c *Chat,
	topology map[string]BranchTopology,
	fn func(Event) bool,
) error {
	currentBranch := c.ActiveBranch
	if currentBranch == "" {
		currentBranch = "main"
	}
	currentCursor := ""

	for {
		continueWalk := true

		err := t.graph.WalkBranchReverse(ctx, c, currentBranch, currentCursor, func(ev Event) bool {
			if !fn(ev) {
				continueWalk = false
				return false
			}
			return true
		})
		if err != nil {
			return err
		}

		if !continueWalk {
			break
		}

		topo, exists := topology[currentBranch]
		if !exists || topo.ParentBranch == "" {
			break
		}
		currentBranch = topo.ParentBranch
		currentCursor = topo.ForkPoint
	}

	return nil
}

func (t *TraversalEngine) GetFullTimeline(ctx context.Context, c *Chat, topology map[string]BranchTopology) ([]Event, error) {
	var allEvents []Event
	err := t.WalkTimelineReverse(ctx, c, topology, func(ev Event) bool {
		allEvents = append(allEvents, ev)
		return true
	})
	if err != nil {
		return nil, err
	}

	// Walk returns newest first, so we reverse it to maintain chronological order
	for i, j := 0, len(allEvents)-1; i < j; i, j = i+1, j-1 {
		allEvents[i], allEvents[j] = allEvents[j], allEvents[i]
	}
	return allEvents, nil
}
