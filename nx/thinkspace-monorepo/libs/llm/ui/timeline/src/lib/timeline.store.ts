import { Injectable, computed, inject, signal } from '@angular/core';
import { ChatStateService } from '@org/llm-state-chat';

export type TimelineViewMode = 'linear' | 'cognitive';

@Injectable({ providedIn: 'root' })
export class TimelineStore {
  private chatState = inject(ChatStateService);

  public viewMode = signal<TimelineViewMode>('linear');
  
  // Passed straight through to Dumb Components
  public flowStates = this.chatState.flowStates;
  public rawFeed = this.chatState.coreChat;
  public activeDigests = this.chatState.activeDigests;
  public activeFocus = this.chatState.activeFocus;
  
  // For the Cognitive Timeline, we only want to show raw messages that 
  // haven't been swallowed by the background digestion process yet.
  public workingMemoryFeed = computed(() => {
    // In a fully robust implementation, we would compare message timestamps 
    // or ledger IDs against the DigestionHorizon ID.
    // Since our backend facade currently reconstructs coreChat from the 
    // `recentEvents` array (which represents exactly what is in front of the horizon),
    // we can pass this directly as the active working memory.
    return this.chatState.coreChat();
  });

  public toggleViewMode() {
    this.viewMode.update(m => m === 'linear' ? 'cognitive' : 'linear');
  }

  public inspectFlow(flowId: string) {
    this.chatState.inspectFlow(flowId);
  }
}