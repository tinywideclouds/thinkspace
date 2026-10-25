import { Injectable, computed, inject, signal } from '@angular/core';
import { TimelineStore, FlowStore, CognitiveStore, ChatItem, WorkspaceStateService } from '@org/llm-state-chat';

export type TimelineViewMode = 'linear' | 'cognitive';

@Injectable({ providedIn: 'root' })
export class TimelineFacade {
  private domainTimeline = inject(TimelineStore);
  private flow = inject(FlowStore);
  private cognitive = inject(CognitiveStore);
  private workspace = inject(WorkspaceStateService);

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
    const bundledFeed: any[] = [];
    const attachmentMap = new Map<string, any[]>();

    // Pass 1: Extract attachments linked by parent_id (handling both flat and nested metadata)
    for (const item of feed) {
      const parentId = (item as any).parentId || (item as any).metadata?.parent_id;
      if (parentId) {
        if (!attachmentMap.has(parentId)) attachmentMap.set(parentId, []);
        attachmentMap.get(parentId)!.push(item);
      }
    }

    // Pass 2: Build the timeline, skipping items that were bundled as attachments
    for (let i = 0; i < feed.length; i++) {
      const item = feed[i];
      const parentId = (item as any).parentId || (item as any).metadata?.parent_id;
      if (parentId) continue;

      const enrichedItem: any = { ...item, attachments: attachmentMap.get(item.id) || [] };

      // Tag merging logic
      if (item.type === 'user') {
        const nextItem = feed[i + 1];
        if (nextItem && (nextItem.type === 'model' || nextItem.type === 'system') && nextItem.tags) {
          enrichedItem.tags = Array.from(new Set([...(item.tags || []), ...nextItem.tags]));
        }
      }
      
      bundledFeed.push(enrichedItem);
    }

    return bundledFeed;
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

  public getFileContent(path: string, ref: string): Promise<string> {
    return this.workspace.getFileContent(path, ref);
  }
}