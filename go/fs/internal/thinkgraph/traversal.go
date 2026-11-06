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
	c *ChatAddress,
	cursor string,
	limit int,
	topology map[string]BranchTopology,
) ([]Node, string, bool, error) {

	var finalNodes []Node
	currentName := c.Name
	if currentName == "" {
		currentName = "main"
	}
	currentGraphID := c.GraphID
	currentCursor := cursor
	remainingLimit := limit

	for remainingLimit > 0 {
		nodes, nextCursor, hasMore, err := t.graph.GetSubGraphTimelinePage(ctx, c, currentGraphID, currentCursor, remainingLimit)
		if err != nil {
			return nil, "", false, err
		}

		finalNodes = append(nodes, finalNodes...) // Prepend since Badger returns oldest-first within the limit block
		remainingLimit -= len(nodes)

		if hasMore {
			return finalNodes, nextCursor, true, nil
		}

		// Reached the end of the current branch. Hop to the parent if it exists.
		topo, exists := topology[currentName]
		if !exists || topo.ParentBranch == "" {
			break // Root of the entire chat
		}

		currentName = topo.ParentBranch
		currentGraphID = GraphID{} // Parent graph ID is inherently unknown to string-based topology map
		currentCursor = topo.ForkPoint
	}

	hasMoreTotal := false
	var finalCursor string
	if remainingLimit == 0 {
		hasMoreTotal = true
		if len(finalNodes) > 0 {
			finalCursor = finalNodes[0].ID.String()
		}
	}

	return finalNodes, finalCursor, hasMoreTotal, nil
}

func (t *TraversalEngine) WalkTimelineReverse(
	ctx context.Context,
	c *ChatAddress,
	topology map[string]BranchTopology,
	fn func(Node) bool,
) error {
	currentName := c.Name
	if currentName == "" {
		currentName = "main"
	}
	currentGraphID := c.GraphID
	currentCursor := ""

	for {
		continueWalk := true

		err := t.graph.WalkSubGraphReverse(ctx, c, currentGraphID, currentCursor, func(node Node) bool {
			if !fn(node) {
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

		topo, exists := topology[currentName]
		if !exists || topo.ParentBranch == "" {
			break
		}
		currentName = topo.ParentBranch
		currentGraphID = GraphID{}
		currentCursor = topo.ForkPoint
	}

	return nil
}

func (t *TraversalEngine) GetFullTimeline(ctx context.Context, c *ChatAddress, topology map[string]BranchTopology) ([]Node, error) {
	var allNodes []Node
	err := t.WalkTimelineReverse(ctx, c, topology, func(node Node) bool {
		allNodes = append(allNodes, node)
		return true
	})
	if err != nil {
		return nil, err
	}

	// Walk returns newest first, so we reverse it to maintain chronological order
	for i, j := 0, len(allNodes)-1; i < j; i, j = i+1, j-1 {
		allNodes[i], allNodes[j] = allNodes[j], allNodes[i]
	}
	return allNodes, nil
}
