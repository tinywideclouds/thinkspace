import { Component, input, computed, inject, effect } from '@angular/core';
import { CommonModule } from '@angular/common';
import { GraphStore, ChatConnectionService } from '@org/llm-state-chat';
import { GraphCanvasComponent } from '@org/llm-ui-graph';

interface LayoutNode {
  id: string;
  x: number;
  y: number;
  data: any;
}

interface LayoutEdge {
  id: string;
  pathData: string;
}

@Component({
  selector: 'llm-flow-graph',
  standalone: true,
  imports: [CommonModule, GraphCanvasComponent],
  templateUrl: './flow-graph.component.html',
  styleUrl: './flow-graph.component.scss'
})
export class FlowGraphComponent {
  private graphStore = inject(GraphStore);
  private connection = inject(ChatConnectionService);

  public flowId = input.required<string>();

  constructor() {
    // Automatically fetch the bounded subgraph data when this component mounts
    effect(() => {
      const id = this.flowId();
      if (id) {
        this.connection.loadSubGraph(id);
      }
    });
  }

  // Pure mathematical projection from the universal GraphStore
  public layout = computed(() => {
    const targetFlowId = this.flowId();
    const allNodes = Array.from(this.graphStore.nodes().values());
    
    // 1. Filter for this specific DAG
    const flowNodes = allNodes
      .filter(n => n.branchId === targetFlowId && !n.tombstoned)
      .sort((a, b) => a.timestamp.toString().localeCompare(b.timestamp.toString()));

    if (flowNodes.length === 0) {
      return { nodes: [], edges: [] };
    }

    // 2. Build adjacency list and find root(s)
    const childrenMap = new Map<string, string[]>();
    const isChild = new Set<string>();

    for (const node of flowNodes) {
      const parentId = node.edges?.['parent_id'];
      if (parentId) {
        if (!childrenMap.has(parentId)) childrenMap.set(parentId, []);
        childrenMap.get(parentId)!.push(node.id);
        isChild.add(node.id);
      }
    }

    const roots = flowNodes.filter(n => !isChild.has(n.id));
    
    // 3. Simple depth-first coordinate calculation (MVP Grid Layout)
    const NODE_WIDTH = 280;
    const NODE_HEIGHT = 100;
    const GAP_X = 50;
    const GAP_Y = 80;

    const layoutNodes: LayoutNode[] = [];
    let currentX = 0;

    const traverse = (nodeId: string, depthY: number) => {
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
        // Only increment X when we hit a leaf, keeping branches spread horizontally
        currentX += NODE_WIDTH + GAP_X;
      }
    };

    for (const root of roots) {
      traverse(root.id, 0);
    }

    // 4. Calculate SVG Bezier Paths for Edges
    const layoutEdges: LayoutEdge[] = [];
    for (const node of layoutNodes) {
      const parentId = node.data.edges?.['parent_id'];
      if (parentId) {
        const parentNode = layoutNodes.find(n => n.id === parentId);
        if (parentNode) {
          // Draw from bottom-center of parent to top-center of child
          const startX = parentNode.x + (NODE_WIDTH / 2);
          const startY = parentNode.y + NODE_HEIGHT;
          const endX = node.x + (NODE_WIDTH / 2);
          const endY = node.y;

          // Simple smooth cubic bezier curve
          const controlY = startY + (endY - startY) / 2;
          const pathData = `M ${startX} ${startY} C ${startX} ${controlY}, ${endX} ${controlY}, ${endX} ${endY}`;

          layoutEdges.push({ id: `${parentId}->${node.id}`, pathData });
        }
      }
    }

    return { nodes: layoutNodes, edges: layoutEdges };
  });
}