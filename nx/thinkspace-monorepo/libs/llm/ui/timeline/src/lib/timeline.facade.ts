import { Injectable, computed, inject, signal, effect } from '@angular/core';
import { TimelineStore, FlowStore, CognitiveStore, ChatItem, WorkspaceStateService, UiSettingsService, FlowExecutionChatItem } from '@org/llm-state-chat';

export type TimelineViewMode = 'linear' | 'cognitive';

export interface Turn {
  id: string;
  leader: ChatItem;
  blocks: ChatItem[];
  tags: string[];
}

export type EnrichedChatItem = ChatItem & { attachments: ChatItem[] };

@Injectable({ providedIn: 'root' })
export class TimelineFacade {
  private domainTimeline = inject(TimelineStore);
  private flow = inject(FlowStore);
  private cognitive = inject(CognitiveStore);
  private workspace = inject(WorkspaceStateService);
  public uiSettings = inject(UiSettingsService);

  public viewMode = signal<TimelineViewMode>('linear');
  public isDebugMode = signal<boolean>(true);
  public isEditMode = signal<boolean>(false);
  
  public hoveredTag = signal<string | null>(null);
  public selectedTag = signal<string | null>(null);
  
  public selectedTurnIds = signal<Set<string>>(new Set());
  public activeTagFilters = signal<Set<string>>(new Set());

  public flowStates = this.flow.flowStates;
  // Point directly to the coreChat computed array projection
  public rawFeed = this.domainTimeline.coreChat; 
  public activeDigests = this.cognitive.activeDigests;
  public activeFocus = this.cognitive.activeFocus;
  
  public isLoadingHistory = this.domainTimeline.isLoadingHistory;
  public paginationSettings = this.uiSettings.pagination;

  // --- Exposed Interactive Prompt Signals ---
  public inspectedFlow = this.flow.inspectedFlow;
  public pendingStrategy = this.cognitive.pendingStrategy;
  public pendingReviewBranch = this.cognitive.pendingReviewBranch;
  public pendingTransition = this.cognitive.pendingTransition;

  constructor() {
    // ---> ADDED LOGGING HERE <---
    effect(() => {
      const raw = this.rawFeed();
      const visible = this.visibleFeed();
      
      console.log(`[TimelineFacade DEBUG] Projection Updated:`);
      console.log(`  -> Raw Graph Nodes (main branch): ${raw.length}`);
      console.log(`  -> Visible Turns Grouped: ${visible.length}`);
      
      if (raw.length > 0) {
        const lastNode = raw[raw.length - 1];
        console.log(`  -> Last Node ID: ${lastNode.id}, Type: ${lastNode.type}, Content Length: ${lastNode.content?.length || 0}`);
      }
    });
  }

  public enhancedFeed = computed<EnrichedChatItem[]>(() => {
    const feed = this.rawFeed();
    const bundledFeed: EnrichedChatItem[] = [];
    const attachmentMap = new Map<string, ChatItem[]>();

    const getParentId = (item: ChatItem): string | undefined => {
      return item.type === 'file_delta' ? item.parentId : undefined;
    };

    for (const item of feed) {
      const parentId = getParentId(item);
      if (parentId) {
        if (!attachmentMap.has(parentId)) attachmentMap.set(parentId, []);
        attachmentMap.get(parentId)?.push(item);
      }
    }

    for (let i = 0; i < feed.length; i++) {
      const item = feed[i];
      if (getParentId(item)) continue;

      const enrichedItem: EnrichedChatItem = { 
        ...item, 
        attachments: attachmentMap.get(item.id) || [] 
      };

      if (enrichedItem.type === 'prompt') {
        const nextItem = feed[i + 1];
        if (nextItem && (nextItem.type === 'model' || nextItem.type === 'system') && nextItem.tags) {
          enrichedItem.tags = Array.from(new Set([...(enrichedItem.tags || []), ...nextItem.tags]));
        }
      }
      
      bundledFeed.push(enrichedItem);
    }

    return bundledFeed;
  });

  public visibleFeed = computed<Turn[]>(() => {
    const feed = this.enhancedFeed();
    const filters = this.activeTagFilters();
    const filterMode = this.uiSettings.tagFilterMode();
    
    const turns: Turn[] = [];
    let currentTurn: Turn | null = null;

    for (const item of feed) {
      if (item.type === 'prompt' || !currentTurn) {
        if (currentTurn) turns.push(currentTurn);
        currentTurn = {
          id: item.id,
          leader: item,
          blocks: [],
          tags: item.tags || []
        };
        if (item.type !== 'prompt') {
          currentTurn.blocks.push(item);
        }
      } else {
        currentTurn.blocks.push(item);
        if (item.tags) {
          currentTurn.tags = Array.from(new Set([...currentTurn.tags, ...item.tags]));
        }
      }
    }
    if (currentTurn) turns.push(currentTurn);

    if (filterMode === 'visual' && filters.size > 0) {
      return turns.filter(turn => turn.tags.some(tag => filters.has(tag)));
    }

    return turns;
  });

  public toggleViewMode(): void {
    this.viewMode.update(m => m === 'linear' ? 'cognitive' : 'linear');
  }

  public toggleDebugMode(): void {
    this.isDebugMode.update(m => !m);
  }

  public toggleEditMode(): void {
    this.isEditMode.update(m => !m);
    if (!this.isEditMode()) {
      this.selectedTurnIds.set(new Set());
    }
  }

  public inspectFlow(item: FlowExecutionChatItem): void {
    this.flow.inspectFlow(item);
  }

  public viewGraph(flowId: string): void {
    this.flow.inspectedGraphId.set(flowId);
  }

  public closeInspector(): void {
    this.flow.inspectedFlow.set(null);
  }

  public setHoveredTag(tag: string | null): void {
    this.hoveredTag.set(tag);
  }

  public toggleSelectedTag(tag: string): void {
    this.selectedTag.update(current => current === tag ? null : tag);
  }

  public toggleTurnSelection(turnId: string): void {
    this.selectedTurnIds.update(set => {
      const newSet = new Set(set);
      if (newSet.has(turnId)) newSet.delete(turnId);
      else newSet.add(turnId);
      return newSet;
    });
  }

  public toggleTagFilter(tag: string): void {
    this.activeTagFilters.update(set => {
      const newSet = new Set(set);
      if (newSet.has(tag)) newSet.delete(tag);
      else newSet.add(tag);
      return newSet;
    });

    if (this.uiSettings.tagFilterMode() === 'context') {
      this.dispatchContextFocus();
    }
  }

  public dispatchContextFocus(): void {
    const tags = Array.from(this.activeTagFilters());
    this.cognitive.activeFocus.update(focus => ({
      primaryFocus: tags,
      relatedContext: focus?.relatedContext || [],
      dormantLenses: focus?.dormantLenses || []
    }));
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