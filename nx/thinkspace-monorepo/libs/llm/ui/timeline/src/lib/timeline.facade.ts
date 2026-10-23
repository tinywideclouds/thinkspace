import { Injectable, computed, inject, signal } from '@angular/core';
import { TimelineStore, FlowStore, CognitiveStore, ChatItem } from '@org/llm-state-chat';

export type TimelineViewMode = 'linear' | 'cognitive';

@Injectable({ providedIn: 'root' })
export class TimelineFacade {
  private domainTimeline = inject(TimelineStore);
  private flow = inject(FlowStore);
  private cognitive = inject(CognitiveStore);

  public viewMode = signal<TimelineViewMode>('linear');
  public isDebugMode = signal<boolean>(true);
  
  public hoveredTag = signal<string | null>(null);
  public selectedTag = signal<string | null>(null);

  public flowStates = this.flow.flowStates;
  public rawFeed = this.domainTimeline.coreChat;
  public activeDigests = this.cognitive.activeDigests;
  public activeFocus = this.cognitive.activeFocus;
  
  public enhancedFeed = computed<ChatItem[]>(() => {
    const feed = this.rawFeed();
    return feed.map((item, index) => {
      if (item.source === 'user') {
        const nextItem = feed[index + 1];
        if (nextItem && (nextItem.source === 'model' || nextItem.source === 'system') && nextItem.tags) {
          return { 
            ...item, 
            tags: Array.from(new Set([...(item.tags || []), ...nextItem.tags])) 
          };
        }
      }
      return item;
    });
  });

  public workingMemoryFeed = computed(() => {
    return this.enhancedFeed();
  });

  public toggleViewMode(): void {
    this.viewMode.update(m => m === 'linear' ? 'cognitive' : 'linear');
  }

  public toggleDebugMode(): void {
    this.isDebugMode.update(m => !m);
  }

  public inspectFlow(flowId: string): void {
    this.flow.inspectFlow(flowId);
  }

  public setHoveredTag(tag: string | null): void {
    this.hoveredTag.set(tag);
  }

  public toggleSelectedTag(tag: string): void {
    this.selectedTag.update(current => current === tag ? null : tag);
  }

  public isItemDimmed(item: ChatItem): boolean {
    const activeTag = this.hoveredTag() || this.selectedTag();
    if (!activeTag) return false;
    
    if (!item.tags || item.tags.length === 0) return true;
    return !item.tags.includes(activeTag);
  }
  
  public isItemCollapsed(item: ChatItem): boolean {
    const selected = this.selectedTag();
    if (!selected) return false; 
    
    if (!item.tags || item.tags.length === 0) return true;
    return !item.tags.includes(selected);
  }
}