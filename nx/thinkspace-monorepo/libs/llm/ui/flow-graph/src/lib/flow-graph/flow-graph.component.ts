import { Component, input, computed, inject, effect } from '@angular/core';
import { CommonModule } from '@angular/common';
import { GraphStore, ChatConnectionService, ThinkspaceStateService, ChatItem } from '@org/llm-state-chat';
import { GraphCanvasComponent } from '@org/llm-ui-graph';

export interface LayoutNode {
  id: string;
  x: number;
  y: number;
  data: ChatItem;
}

export interface LayoutEdge {
  id: string;
  pathData: string;
}

@Component({
  selector: 'llm-flow-graph',
  standalone: true,
  imports: [CommonModule, GraphCanvasComponent],
  templateUrl: './flow-graph.component.html',
  styleUrl: './flow-graph.component.scss',
  host: {
    class: 'block w-full h-full'
  }
})
export class FlowGraphComponent {
  private graphStore = inject(GraphStore);
  private connection = inject(ChatConnectionService);
  private workspace = inject(ThinkspaceStateService);

  public flowId = input.required<string>();

  constructor() {
    effect(() => {
      const id = this.flowId();
      if (id) {
        this.connection.loadSubGraph(id);
      }
    });
  }

  public layout = computed(() => {
    const targetFlowId = this.flowId();
    const spaceId = this.workspace.activeSpaceId();
    const chatId = this.workspace.activeChatId();

    if (!spaceId || !chatId || !targetFlowId) {
      return { nodes: [], edges: [] };
    }

    const key = GraphStore.makeKey({ spaceId, chatId, projectionId: targetFlowId });
    const pointers = this.graphStore.projections().get(key) || [];
    const cache = this.graphStore.nodes();

    // 1. Initial mapping of local subgraph nodes[cite: 5]
    const localNodes = pointers
      .map(id => cache.get(id))
      .filter((n): n is ChatItem => n !== undefined && !n.tombstoned);

    if (localNodes.length === 0) {
      return { nodes: [], edges: [] };
    }

    const flowNodesMap = new Map<string, ChatItem>(localNodes.map(n => [n.id, n]));

    // 2. Cross-Boundary Resolution: Pull immediate cross-boundary parents and children from global cache[cite: 5]
    let added = true;
    while (added) {
      added = false;

      // Resolve parents referenced by nodes already in the graph (e.g., Manager Node or Flow Options Node)[cite: 5]
      for (const node of Array.from(flowNodesMap.values())) {
        const edges = node.edges || {};
        for (const k of Object.keys(edges)) {
          if (k.startsWith('parent')) {
            const parentId = edges[k];
            if (!flowNodesMap.has(parentId)) {
              const parentNode = cache.get(parentId);
              if (parentNode && !parentNode.tombstoned) {
                flowNodesMap.set(parentId, parentNode);
                added = true;
              }
            }
          }
        }
      }

      // Resolve bridge children pointing into local subgraph nodes (e.g., Flow Options Node pointing to Join Node)[cite: 5]
      for (const node of cache.values()) {
        if (node.tombstoned || flowNodesMap.has(node.id)) continue;
        const edges = node.edges || {};
        const pointsToLocal = Object.keys(edges).some(
          k => k.startsWith('parent') && localNodes.some(ln => ln.id === edges[k])
        );
        if (pointsToLocal) {
          flowNodesMap.set(node.id, node);
          added = true;
        }
      }
    }

    const flowNodes = Array.from(flowNodesMap.values());

    // 3. Build Adjacency & Parent Maps[cite: 5]
    const childrenMap = new Map<string, string[]>();
    const parentsMap = new Map<string, string[]>();

    for (const node of flowNodes) {
      const edges = node.edges || {};
      const validParents = Object.keys(edges)
        .filter(k => k.startsWith('parent'))
        .map(k => edges[k])
        .filter(pid => flowNodesMap.has(pid));

      parentsMap.set(node.id, validParents);
      for (const pid of validParents) {
        if (!childrenMap.has(pid)) childrenMap.set(pid, []);
        childrenMap.get(pid)?.push(node.id);
      }
    }

    // 4. Compute Longest-Path Depth (ensures diamond join nodes always render below their deepest branch)[cite: 5]
    const depthMap = new Map<string, number>();
    const computing = new Set<string>();

    const getDepth = (nodeId: string): number => {
      if (depthMap.has(nodeId)) return depthMap.get(nodeId)!;
      if (computing.has(nodeId)) return 0; // Cycle guard[cite: 5]

      computing.add(nodeId);
      const parents = parentsMap.get(nodeId) || [];
      let depth = 0;
      if (parents.length > 0) {
        depth = Math.max(...parents.map(pid => getDepth(pid))) + 1;
      }
      computing.delete(nodeId);
      depthMap.set(nodeId, depth);
      return depth;
    };

    for (const node of flowNodes) {
      getDepth(node.id);
    }

    // 5. Assign X Coordinates via Depth-Layered DAG Layout
    const NODE_WIDTH = 280;
    const NODE_HEIGHT = 100;
    const GAP_X = 50;
    const GAP_Y = 80;
    const STEP_X = NODE_WIDTH + GAP_X;

    const layers = new Map<number, ChatItem[]>();
    let maxDepth = 0;
    for (const node of flowNodes) {
      const d = depthMap.get(node.id) ?? 0;
      if (d > maxDepth) maxDepth = d;
      if (!layers.has(d)) layers.set(d, []);
      layers.get(d)!.push(node);
    }

    const xMap = new Map<string, number>();

    // Pass A (Top-Down): Position nodes using parent barycenter, resolving horizontal collisions per layer
    for (let d = 0; d <= maxDepth; d++) {
      const layerNodes = layers.get(d) || [];

      const desired = layerNodes.map((node, index) => {
        const parents = parentsMap.get(node.id) || [];
        const parentXs = parents
          .map(pid => xMap.get(pid))
          .filter((x): x is number => x !== undefined);

        const targetX = parentXs.length > 0
          ? parentXs.reduce((sum, x) => sum + x, 0) / parentXs.length
          : index * STEP_X;

        const agentTag = node.metadata?.['agent_id'] || '';
        return { node, targetX, agentTag, index };
      });

      // Preserve deterministic branch ordering (agent-1 left, agent-2 right, etc.)
      desired.sort((a, b) => {
        if (Math.abs(a.targetX - b.targetX) > 1) {
          return a.targetX - b.targetX;
        }
        if (a.agentTag !== b.agentTag) {
          return a.agentTag.localeCompare(b.agentTag);
        }
        return a.index - b.index;
      });

      // Spread siblings cleanly without overlap, then center the group around their average desired X
      const positioned: { id: string; x: number }[] = [];
      let nextMinX = -Infinity;
      for (const item of desired) {
        const x = Math.max(item.targetX, nextMinX);
        positioned.push({ id: item.node.id, x });
        nextMinX = x + STEP_X;
      }

      if (positioned.length > 0) {
        const avgDesired = desired.reduce((s, i) => s + i.targetX, 0) / desired.length;
        const avgActual = positioned.reduce((s, p) => s + p.x, 0) / positioned.length;
        const shift = avgDesired - avgActual;
        for (const p of positioned) {
          xMap.set(p.id, p.x + shift);
        }
      }
    }

    // Pass B (Bottom-Up): Center fork parents (with multiple children) over their spread children
    for (let d = maxDepth - 1; d >= 0; d--) {
      const layerNodes = layers.get(d) || [];
      if (layerNodes.length === 1) {
        const onlyNode = layerNodes[0];
        const children = childrenMap.get(onlyNode.id) || [];
        if (children.length > 1) {
          const childXs = children
            .map(cid => xMap.get(cid))
            .filter((x): x is number => x !== undefined);

          if (childXs.length > 1) {
            const minX = Math.min(...childXs);
            const maxX = Math.max(...childXs);
            xMap.set(onlyNode.id, (minX + maxX) / 2);
          }
        }
      }
    }

    // Normalize X coordinates so the leftmost node starts at x = 0
    const allXs = Array.from(xMap.values());
    const minCanvasX = allXs.length > 0 ? Math.min(...allXs) : 0;

    const layoutNodes: LayoutNode[] = flowNodes.map(node => ({
      id: node.id,
      x: (xMap.get(node.id) ?? 0) - minCanvasX,
      y: (depthMap.get(node.id) ?? 0) * (NODE_HEIGHT + GAP_Y),
      data: node
    }));

    const layoutNodeMap = new Map<string, LayoutNode>(layoutNodes.map(n => [n.id, n]));

    // 6. Generate Bezier Edge Paths[cite: 5]
    const layoutEdges: LayoutEdge[] = [];
    for (const node of layoutNodes) {
      const parents = parentsMap.get(node.id) || [];
      for (const parentId of parents) {
        const parentNode = layoutNodeMap.get(parentId);
        if (parentNode) {
          const startX = parentNode.x + NODE_WIDTH / 2;
          const startY = parentNode.y + NODE_HEIGHT;
          const endX = node.x + NODE_WIDTH / 2;
          const endY = node.y;

          const controlY = startY + (endY - startY) / 2;
          const pathData = `M ${startX} ${startY} C ${startX} ${controlY}, ${endX} ${controlY}, ${endX} ${endY}`;

          layoutEdges.push({ id: `${parentId}->${node.id}`, pathData });
        }
      }
    }

    return { nodes: layoutNodes, edges: layoutEdges };
  });
}