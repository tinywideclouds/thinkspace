import { Injectable, computed, inject } from '@angular/core';
import { ChatStateService } from '@org/llm-state-chat';

@Injectable({ providedIn: 'root' })
export class CognitiveGraphStore {
  private chatState = inject(ChatStateService);

  // Raw State
  public digests = this.chatState.activeDigests;
  public activeFeed = this.chatState.coreChat;
  public flowStates = this.chatState.flowStates;
  public focus = this.chatState.activeFocus;

  // Derived Graph Topology
  public stickyNodes = computed(() => {
    return Object.values(this.digests()).filter(d => d.isSticky);
  });

  public trajectoryNodes = computed(() => {
    return Object.values(this.digests()).filter(d => !d.isSticky);
  });

  // Actions
  public inspectFlow(flowId: string): void {
    this.chatState.inspectFlow(flowId);
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