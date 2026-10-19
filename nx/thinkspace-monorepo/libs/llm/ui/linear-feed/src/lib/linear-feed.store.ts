import { Injectable, computed, inject, signal } from '@angular/core';
import { ChatStateService } from '@org/llm-state-chat';

@Injectable({ providedIn: 'root' })
export class LinearFeedStore {
  private chatState = inject(ChatStateService);

  // Passthrough signals from global state
  public feed = this.chatState.coreChat;
  public flowStates = this.chatState.flowStates;

  // Local UI State (Ready for future features)
  public activeTagFilter = signal<string | null>(null);

  // Computed feed that will eventually handle filtering/dimming
  public visibleFeed = computed(() => {
    const filter = this.activeTagFilter();
    const currentFeed = this.feed();
    
    if (!filter) return currentFeed;
    
    // Future: Apply dimming/filtering logic based on the tag
    return currentFeed;
  });

  public inspectFlow(flowId: string): void {
    this.chatState.inspectFlow(flowId);
  }
}