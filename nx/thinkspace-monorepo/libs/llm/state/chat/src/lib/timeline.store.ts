import { Injectable, signal } from '@angular/core';
import { GraphEvent, BranchTopology } from '@org/llm-core-facade';
import { 
  ChatItem, 
  MessageChatItem, 
  FlowExecutionChatItem, 
  FileDeltaChatItem 
} from './chat.models';

@Injectable({ providedIn: 'root' })
export class TimelineStore {
  public coreChat = signal<ChatItem[]>([]);
  public hasMoreHistory = signal<boolean>(false);
  public nextCursor = signal<string>('');
  public isLoadingHistory = signal<boolean>(false);
  public totalHistoryMessages = signal<number>(0);

  public activeBranch = signal<string>('main');
  public branches = signal<Record<string, BranchTopology>>({});

  public clearSession(): void {
    this.coreChat.set([]);
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

    this.coreChat.update(existing => {
      if (this.isLoadingHistory() && existing.length > 0) {
        return [...incomingTimeline, ...existing];
      }
      return incomingTimeline;
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
    
    const newItem = this.mapGraphEventToChatItem(event);
    
    this.coreChat.update(history => {
      const cleanHistory = history.filter(c => !(c.type === 'system' && c.content.includes('Thinking')));
      return [...cleanHistory, newItem];
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
        status: e.status
      } satisfies FlowExecutionChatItem;
    }

    return { 
      ...base,
      type: e.type, 
      content: e.content
    } satisfies MessageChatItem;
  }

  public appendCoreChat(type: 'prompt' | 'model' | 'system', content: string): void {
    this.coreChat.update(history => {
      const cleanHistory = history.filter(c => !(c.type === 'system' && c.content.includes('Thinking')));
      return [
        ...cleanHistory, 
        { 
          id: crypto.randomUUID(), 
          timestamp: Temporal.Now.instant(), 
          type, 
          content 
        } satisfies MessageChatItem
      ];
    });
  }

  public appendToLastModelMessage(chunkText: string): void {
    this.coreChat.update(history => {
      const cleanHistory = history.filter(c => !(c.type === 'system' && c.content.includes('Thinking')));
      if (cleanHistory.length === 0 || cleanHistory[cleanHistory.length - 1].type !== 'model') {
        return [
          ...cleanHistory, 
          { 
            id: crypto.randomUUID(), 
            timestamp: Temporal.Now.instant(), 
            type: 'model', 
            content: chunkText 
          } satisfies MessageChatItem
        ];
      }
      
      const updated = [...cleanHistory];
      const last = updated[updated.length - 1] as MessageChatItem;
      updated[updated.length - 1] = {
        ...last,
        content: last.content + chunkText
      };
      return updated;
    });
  }

  public injectFlowExecution(flowId: string, summary: string): void {
    this.coreChat.update(history => {
      const cleanHistory = history.filter(c => !(c.type === 'system' && c.content.includes('Thinking')));
      return [
        ...cleanHistory, 
        { 
          id: crypto.randomUUID(), 
          timestamp: Temporal.Now.instant(), 
          type: 'flow_execution', 
          content: summary, 
          flowId, 
          status: 'running' 
        } satisfies FlowExecutionChatItem
      ];
    });
  }

  public updateFlowExecutionStatus(flowId: string, status: 'running' | 'completed' | string): void {
    this.coreChat.update(history => history.map(item => {
      if (item.type === 'flow_execution' && item.flowId === flowId) {
        return { ...item, status };
      }
      return item;
    }));
  }

  public clearThinkingIndicators(): void {
    this.coreChat.update(chat => chat.filter(c => !(c.type === 'system' && c.content.includes('Thinking'))));
  }
}