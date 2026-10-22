import { Injectable, computed, inject } from '@angular/core';
import { CognitiveStore, TimelineStore, FlowStore } from '@org/llm-state-chat';

@Injectable({ providedIn: 'root' })
export class CognitiveGraphFacade {
  private cognitive = inject(CognitiveStore);
  private timeline = inject(TimelineStore);
  private flow = inject(FlowStore);

  // Raw State
  public digests = this.cognitive.activeDigests;
  public activeFeed = this.timeline.coreChat;
  public flowStates = this.flow.flowStates;
  public focus = this.cognitive.activeFocus;

  // Derived Graph Topology
  public stickyNodes = computed(() => {
    return Object.values(this.digests()).filter(d => d.isSticky);
  });

  public trajectoryNodes = computed(() => {
    return Object.values(this.digests()).filter(d => !d.isSticky);
  });

  // Actions
  public inspectFlow(flowId: string): void {
    this.flow.inspectFlow(flowId);
  }

  // Node Evaluation
  public isNodeInFocus(summary: string): boolean {
    const focusData = this.focus();
    if (!focusData) return false;
    
    const activeTags = [
      ...(focusData.primaryFocus || []),
      ...(focusData.relatedContext || [])
    ];
    return activeTags.some(tag => summary.includes(`#${tag}`));
  }
}