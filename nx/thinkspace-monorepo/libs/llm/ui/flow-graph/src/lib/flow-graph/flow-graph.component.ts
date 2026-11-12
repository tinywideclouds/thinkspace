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

    // 1. Initial mapping of local subgraph nodes
    const localNodes = pointers
      .map(id => cache.get(id))
      .filter((n): n is ChatItem => n !== undefined && !n.tombstoned);

    if (localNodes.length === 0) {
      return { nodes: [], edges: [] };
    }

    const flowNodesMap = new Map<string, ChatItem>(localNodes.map(n => [n.id, n]));

    // 2. Cross-Boundary Resolution: Pull immediate cross-boundary parents and children from global cache
    let added = true;
    while (added) {
      added = false;

      // Resolve parents referenced by nodes already in the graph (e.g., Manager Node or Flow Options Node)
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

      // Resolve bridge children pointing into local subgraph nodes (e.g., Flow Options Node pointing to Join Node)
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

    // 3. Build Adjacency & Parent Maps
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

    // 4. Compute Longest-Path Depth (ensures diamond join nodes always render below their deepest branch)
    const depthMap = new Map<string, number>();
    const computing = new Set<string>();

    const getDepth = (nodeId: string): number => {
      if (depthMap.has(nodeId)) return depthMap.get(nodeId)!;
      if (computing.has(nodeId)) return 0; // Cycle guard

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

    // 5. Assign X Coordinates via DFS Leaf-Spread & Center Parents Over Children
    const NODE_WIDTH = 280;
    const NODE_HEIGHT = 100;
    const GAP_X = 50;
    const GAP_Y = 80;

    const xMap = new Map<string, number>();
    const visited = new Set<string>();
    let currentX = 0;

    const assignX = (nodeId: string): number => {
      if (visited.has(nodeId)) {
        return xMap.get(nodeId) ?? currentX;
      }
      visited.add(nodeId);

      const children = childrenMap.get(nodeId) || [];
      if (children.length === 0) {
        const x = currentX;
        currentX += NODE_WIDTH + GAP_X;
        xMap.set(nodeId, x);
        return x;
      }

      const childXs = children.map(cid => assignX(cid));
      const minX = Math.min(...childXs);
      const maxX = Math.max(...childXs);
      const centerX = (minX + maxX) / 2;
      xMap.set(nodeId, centerX);
      return centerX;
    };

    const roots = flowNodes.filter(n => (parentsMap.get(n.id) || []).length === 0);
    for (const root of roots) {
      assignX(root.id);
    }
    // Catch any disconnected nodes
    for (const node of flowNodes) {
      if (!visited.has(node.id)) {
        assignX(node.id);
      }
    }

    const layoutNodes: LayoutNode[] = flowNodes.map(node => ({
      id: node.id,
      x: xMap.get(node.id) ?? 0,
      y: (depthMap.get(node.id) ?? 0) * (NODE_HEIGHT + GAP_Y),
      data: node
    }));

    const layoutNodeMap = new Map<string, LayoutNode>(layoutNodes.map(n => [n.id, n]));

    // 6. Generate Bezier Edge Paths
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