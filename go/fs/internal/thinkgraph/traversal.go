package thinkgraph

import (
	"context"
	"encoding/json"
	"fmt"

	"uuid"
)

// TraversalEngine safely stitches timelines across branches by tracing graph-native branch anchors.
type TraversalEngine struct {
	graph GraphRepository
}

func NewTraversalEngine(graph GraphRepository) *TraversalEngine {
	return &TraversalEngine{graph: graph}
}

// ResolveSubGraphID dynamically resolves a short flow alias (e.g. "flow-1234") into its strictly
// mathematically indexed UUID location on the graph, avoiding abstraction leaks in the API transport layer.
func (t *TraversalEngine) ResolveSubGraphID(ctx context.Context, c *ChatAddress, alias string) (GraphID, error) {
	var targetGraphID GraphID

	err := t.WalkTimelineReverse(ctx, c, func(n Node) bool {
		if string(n.Type) == "flow_lifecycle" {
			var config struct {
				FlowID string `json:"flow_id"`
			}
			if json.Unmarshal(n.Data, &config) == nil && config.FlowID == alias {
				targetGraphID = GraphID{Prefix: PrefixFlow, UUID: n.ID}
				return false
			}
		}
		return true
	})

	if err != nil {
		return GraphID{}, err
	}

	if targetGraphID.IsZero() {
		return GraphID{}, fmt.Errorf("subgraph alias %q not found on timeline", alias)
	}

	return targetGraphID, nil
}

func (t *TraversalEngine) resolveParentGraphID(ctx context.Context, c *ChatAddress, forkPoint uuid.UUID) (GraphID, error) {
	if forkPoint == uuid.Nil() {
		return GraphID{}, nil
	}
	nodes, err := t.graph.GetNodes(ctx, c, []string{forkPoint.String()})
	if err != nil {
		return GraphID{}, fmt.Errorf("resolving parent graph ID for fork point %s: %w", forkPoint, err)
	}
	if len(nodes) > 0 {
		return nodes[0].GraphID, nil
	}
	return GraphID{}, nil
}

func (t *TraversalEngine) GetTimelinePage(
	ctx context.Context,
	c *ChatAddress,
	cursor string,
	limit int,
) ([]Node, string, bool, error) {
	var finalNodes []Node
	currentName := c.Name
	if currentName == "" {
		currentName = "main"
	}
	currentGraphID := c.GraphID
	currentCursor := cursor
	remainingLimit := limit
	hasMoreParent := false
	var nextParentCursor string

	for remainingLimit > 0 {
		var parentBranch string
		var forkPoint uuid.UUID

		nodes, nextCursor, hasMore, err := t.graph.GetSubGraphTimelinePage(ctx, &ChatAddress{ChatID: c.ChatID, ProjectID: c.ProjectID, SubGraph: SubGraph{Name: currentName}}, currentGraphID, currentCursor, remainingLimit)
		if err != nil {
			return nil, "", false, err
		}

		for _, node := range nodes {
			if node.Type == EventBranchAnchor {
				var anchor BranchAnchor
				if unmarshalErr := json.Unmarshal(node.Data, &anchor); unmarshalErr != nil {
					return nil, "", false, fmt.Errorf("unmarshaling branch anchor on node %s: %w", node.ID, unmarshalErr)
				}
				parentBranch = anchor.ParentBranch
				forkPoint = anchor.ForkPoint
			}
		}

		finalNodes = append(nodes, finalNodes...)
		remainingLimit -= len(nodes)

		if hasMore {
			return finalNodes, nextCursor, true, nil
		}

		if parentBranch == "" {
			hasMoreParent = false
			break
		}

		if remainingLimit == 0 {
			hasMoreParent = true
			nextParentCursor = forkPoint.String()
			break
		}

		parentGraphID, err := t.resolveParentGraphID(ctx, c, forkPoint)
		if err != nil {
			return nil, "", false, err
		}

		currentName = parentBranch
		currentGraphID = parentGraphID
		currentCursor = forkPoint.String()
	}

	if hasMoreParent {
		return finalNodes, nextParentCursor, true, nil
	}

	return finalNodes, "", false, nil
}

func (t *TraversalEngine) WalkTimelineReverse(
	ctx context.Context,
	c *ChatAddress,
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
		var parentBranch string
		var forkPoint uuid.UUID
		var walkErr error

		err := t.graph.WalkSubGraphReverse(ctx, &ChatAddress{ChatID: c.ChatID, ProjectID: c.ProjectID, SubGraph: SubGraph{Name: currentName}}, currentGraphID, currentCursor, func(node Node) bool {
			if node.Type == EventBranchAnchor {
				var anchor BranchAnchor
				if unmarshalErr := json.Unmarshal(node.Data, &anchor); unmarshalErr != nil {
					walkErr = fmt.Errorf("unmarshaling branch anchor on node %s: %w", node.ID, unmarshalErr)
					return false
				}
				parentBranch = anchor.ParentBranch
				forkPoint = anchor.ForkPoint
			}

			if !fn(node) {
				continueWalk = false
				return false
			}
			return true
		})
		if err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}

		if !continueWalk || parentBranch == "" {
			break
		}

		parentGraphID, err := t.resolveParentGraphID(ctx, c, forkPoint)
		if err != nil {
			return err
		}

		currentName = parentBranch
		currentGraphID = parentGraphID
		currentCursor = forkPoint.String()
	}

	return nil
}

func (t *TraversalEngine) GetFullTimeline(ctx context.Context, c *ChatAddress) ([]Node, error) {
	var allNodes []Node
	err := t.WalkTimelineReverse(ctx, c, func(node Node) bool {
		allNodes = append(allNodes, node)
		return true
	})
	if err != nil {
		return nil, err
	}

	for i, j := 0, len(allNodes)-1; i < j; i, j = i+1, j-1 {
		allNodes[i], allNodes[j] = allNodes[j], allNodes[i]
	}
	return allNodes, nil
}
