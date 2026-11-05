import { Injectable, signal, computed } from '@angular/core';
import { GraphEvent, BranchTopology } from '@org/llm-core-facade';
import { 
  ChatItem, 
  MessageChatItem, 
  FlowExecutionChatItem, 
  FileDeltaChatItem 
} from './chat.models';

@Injectable({ providedIn: 'root' })
export class TimelineStore {
  // Normalized state: O(1) lookup for mutations (upserts/tombstones)
  public nodeMap = signal(new Map());

  // UI Projection: strictly chronological array derived from the map
  public coreChat = computed(() => Array.from(this.nodeMap().values()));

  public hasMoreHistory = signal<boolean>(false);
  public nextCursor = signal<string>('');
  public isLoadingHistory = signal<boolean>(false);
  public totalHistoryMessages = signal<number>(0);

  public activeBranch = signal<string>('main');
  public branches = signal<Record<string, BranchTopology>>({});

  public clearSession(): void {
    this.nodeMap.set(new Map());
    this.hasMoreHistory.set(false);
    this.nextCursor.set('');
    this.isLoadingHistory.set(false);
    this.totalHistoryMessages.set(0);
    this.activeBranch.set('main');
    this.branches.set({});
  }

  public processSyncHistory(
    recentEvents: GraphEvent[], 
    nextCursor: string, 
    hasMore: boolean, 
    activeBranch: string,
    branches: Record<string, BranchTopology>,
    totalMessages = 0
  ): void {
    const incomingTimeline: ChatItem[] = recentEvents.map(e => this.mapGraphEventToChatItem(e));

    this.nodeMap.update(existingMap => {
      // Create a fresh map to preserve strict chronological insertion order.
      // Older history blocks come first, then we spread the existing map on top.
      const updatedMap = new Map<string, ChatItem>();
      
      if (this.isLoadingHistory() && existingMap.size > 0) {
        for (const item of incomingTimeline) {
          updatedMap.set(item.id, item);
        }
        for (const [id, item] of existingMap.entries()) {
          updatedMap.set(id, item);
        }
        return updatedMap;
      }

      // If not loading history (e.g. fresh branch switch), just set the incoming items
      for (const item of incomingTimeline) {
        updatedMap.set(item.id, item);
      }
      return updatedMap;
    });

    this.nextCursor.set(nextCursor);
    this.hasMoreHistory.set(hasMore);
    this.totalHistoryMessages.set(totalMessages);
    this.isLoadingHistory.set(false);
    
    this.activeBranch.set(activeBranch);
    this.branches.set(branches);
  }

  public ingestEvent(event: GraphEvent): void {
    if (event.branchId && event.branchId !== this.activeBranch()) return;

    this.nodeMap.update(map => {
      const newMap = new Map(map);

      // Clean up any ephemeral "Thinking..." indicators
      newMap.delete('system-thinking');

      if (event.tombstoned) {
        newMap.delete(event.id);
      } else {
        newMap.set(event.id, this.mapGraphEventToChatItem(event));
      }

      return newMap;
    });
  }

  private mapGraphEventToChatItem(e: GraphEvent): ChatItem {
    const base = {
      id: e.id,
      timestamp: e.timestamp,
      tags: e.tags,
      metadata: e.metadata,
      edges: e.edges,
      branchId: e.branchId
    };

    if (e.type === 'file_delta') {
      return {
        ...base,
        type: 'file_delta',
        content: e.content,
        path: e.path,
        previewSnippet: e.previewSnippet,
        fileRef: e.fileRef,
        parentId: e.parentId,
      } satisfies FileDeltaChatItem;
    }
    
    if (e.type === 'flow_execution') {
      return {
        ...base,
        type: 'flow_execution',
        content: e.content,
        flowId: e.flowId,
        taskId: e.taskId,
        task: e.task,
        status: e.status,
        error: e.error,
        agents: e.agents
      } satisfies FlowExecutionChatItem;
    }

    return { 
      ...base,
      type: e.type, 
      content: e.content
    } satisfies MessageChatItem;
  }

  public appendCoreChat(type: 'prompt' | 'model' | 'system', content: string): void {
    this.nodeMap.update(map => {
      const newMap = new Map(map);
      newMap.delete('system-thinking');

      const id = type === 'system' && content.includes('Thinking') 
        ? 'system-thinking' 
        : crypto.randomUUID();

      newMap.set(id, { 
        id, 
        timestamp: Temporal.Now.instant(), 
        type, 
        content 
      } satisfies MessageChatItem);

      return newMap;
    });
  }

  public appendToLastModelMessage(chunkText: string): void {
    this.nodeMap.update(map => {
      const newMap = new Map(map);
      newMap.delete('system-thinking');

      const keys = Array.from(newMap.keys());
      if (keys.length === 0) {
        const id = crypto.randomUUID();
        newMap.set(id, { 
          id, 
          timestamp: Temporal.Now.instant(), 
          type: 'model', 
          content: chunkText 
        } satisfies MessageChatItem);
        return newMap;
      }

      const lastKey = keys[keys.length - 1];
      const lastItem = newMap.get(lastKey);

      if (lastItem && lastItem.type === 'model') {
        newMap.set(lastKey, {
          ...lastItem,
          content: lastItem.content + chunkText
        } as MessageChatItem);
      } else {
        const id = crypto.randomUUID();
        newMap.set(id, { 
          id, 
          timestamp: Temporal.Now.instant(), 
          type: 'model', 
          content: chunkText 
        } satisfies MessageChatItem);
      }

      return newMap;
    });
  }

  public clearThinkingIndicators(): void {
    this.nodeMap.update(map => {
      const newMap = new Map(map);
      newMap.delete('system-thinking');
      return newMap;
    });
  }
}