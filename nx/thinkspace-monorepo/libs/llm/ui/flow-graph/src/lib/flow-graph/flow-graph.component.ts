import { Component, input, computed, inject, effect } from '@angular/core';
import { CommonModule } from '@angular/common';
import { GraphStore, ChatConnectionService, ThinkspaceStateService, ChatItem } from '@org/llm-state-chat';
import { GraphCanvasComponent } from '@org/llm-ui-graph';

export interface LayoutNode {
  id: string;
  x: number;
  y: number;
  data: any;
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

    // O(1) Cache Join: Instantly maps the subgraph's pointers to pure nodes
    const flowNodes = pointers
      .map(id => cache.get(id))
      .filter((n): n is ChatItem => n !== undefined && !n.tombstoned);

    if (flowNodes.length === 0) {
      return { nodes: [], edges: [] };
    }

    const childrenMap = new Map<string, string[]>();
    const isChild = new Set<string>();

    // Build the DAG dependency mappings by checking explicit cryptographic edges
    for (const node of flowNodes) {
      const edges = node.edges || {};
      const parentIds = Object.keys(edges)
        .filter(k => k.startsWith('parent'))
        .map(k => edges[k]);

      for (const parentId of parentIds) {
        if (!childrenMap.has(parentId)) childrenMap.set(parentId, []);
        childrenMap.get(parentId)?.push(node.id);
        isChild.add(node.id);
      }
    }

    // Roots are nodes with no parents
    const roots = flowNodes.filter(n => !isChild.has(n.id));
    
    const NODE_WIDTH = 280;
    const NODE_HEIGHT = 100;
    const GAP_X = 50;
    const GAP_Y = 80;

    const layoutNodes: LayoutNode[] = [];
    const visited = new Set<string>();
    let currentX = 0;

    // Traverse DAG to assign spatial coordinates
    const traverse = (nodeId: string, depthY: number) => {
      if (visited.has(nodeId)) return;
      visited.add(nodeId);

      const nodeData = flowNodes.find(n => n.id === nodeId);
      if (!nodeData) return;

      const x = currentX;
      const y = depthY * (NODE_HEIGHT + GAP_Y);

      layoutNodes.push({ id: nodeId, x, y, data: nodeData });

      const children = childrenMap.get(nodeId) || [];
      if (children.length > 0) {
        for (const childId of children) {
          traverse(childId, depthY + 1);
        }
      } else {
        // Only advance X-coordinate for leaf nodes (ensuring horizontal spread)
        currentX += NODE_WIDTH + GAP_X;
      }
    };

    for (const root of roots) {
      traverse(root.id, 0);
    }

    // Second pass to generate SVG edge paths using Bezier curves
    const layoutEdges: LayoutEdge[] = [];
    for (const node of layoutNodes) {
      const edges = node.data.edges || {};
      const parentIds = Object.keys(edges)
        .filter(k => k.startsWith('parent'))
        .map(k => edges[k]);

      for (const parentId of parentIds) {
        const parentNode = layoutNodes.find(n => n.id === parentId);
        if (parentNode) {
          const startX = parentNode.x + (NODE_WIDTH / 2);
          const startY = parentNode.y + NODE_HEIGHT;
          const endX = node.x + (NODE_WIDTH / 2);
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