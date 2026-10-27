import { Injectable, signal } from '@angular/core';
import { DomainGraphEvent, DomainBranchTopology } from '@org/llm-core-facade';
import { ChatItem, ModelChatItem, FlowCardChatItem, FileDeltaChatItem } from './chat.models';

@Injectable({ providedIn: 'root' })
export class TimelineStore {
  public coreChat = signal<ChatItem[]>([]);
  public hasMoreHistory = signal<boolean>(false);
  public nextCursor = signal<string>('');
  public isLoadingHistory = signal<boolean>(false);
  public totalHistoryMessages = signal<number>(0);

  // Topology Signals
  public activeBranch = signal<string>('main');
  public branches = signal<Record<string, DomainBranchTopology>>({});

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
    recentEvents: DomainGraphEvent[], 
    nextCursor: string, 
    hasMore: boolean, 
    activeBranch: string,
    branches: Record<string, DomainBranchTopology>,
    totalMessages = 0
  ): void {
    const incomingTimeline: ChatItem[] = recentEvents
      .filter(e => e.type === 'prompt' || e.type === 'model' || e.type === 'file_delta')
      .map(e => this.mapDomainEventToChatItem(e));

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

  public ingestEvent(event: DomainGraphEvent): void {
    // Strict Filtering: Ignore live events that belong to a background branch
    if (event.branchId && event.branchId !== this.activeBranch()) return;

    if (event.type !== 'prompt' && event.type !== 'model' && event.type !== 'file_delta') return;
    
    const newItem = this.mapDomainEventToChatItem(event);
    
    this.coreChat.update(history => {
      const cleanHistory = history.filter(c => !(c.type === 'system' && c.content.includes('Thinking')));
      return [...cleanHistory, newItem];
    });
  }

  private mapDomainEventToChatItem(e: DomainGraphEvent): ChatItem {
    if (e.type === 'file_delta') {
      return {
        id: e.id,
        type: 'file_delta',
        content: e.content,
        path: e.metadata['path'] || 'Unknown Path',
        previewSnippet: e.metadata['previewSnippet'] || '',
        fileRef: e.metadata['fileRef'] || '',
        metadata: e.metadata,
        edges: e.edges
      } as FileDeltaChatItem;
    }
    
    return { 
      id: e.id, 
      type: e.type === 'model' ? 'model' : 'user', 
      content: e.content,
      metadata: e.metadata,
      edges: e.edges
    } as ChatItem;
  }

  public appendCoreChat(type: 'user' | 'model' | 'system', content: string): void {
    this.coreChat.update(history => {
      const cleanHistory = history.filter(c => !(c.type === 'system' && c.content.includes('Thinking')));
      return [...cleanHistory, { id: crypto.randomUUID(), type, content } as ChatItem];
    });
  }

  public appendToLastModelMessage(chunkText: string): void {
    this.coreChat.update(history => {
      const cleanHistory = history.filter(c => !(c.type === 'system' && c.content.includes('Thinking')));
      if (cleanHistory.length === 0 || cleanHistory[cleanHistory.length - 1].type !== 'model') {
        return [...cleanHistory, { id: crypto.randomUUID(), type: 'model', content: chunkText } as ModelChatItem];
      }
      const updated = [...cleanHistory];
      updated[updated.length - 1].content += chunkText;
      return updated;
    });
  }

  public injectFlowCard(flowId: string, summary: string): void {
    this.coreChat.update(history => {
      const cleanHistory = history.filter(c => !(c.type === 'system' && c.content.includes('Thinking')));
      return [...cleanHistory, { id: crypto.randomUUID(), type: 'flow_card', content: summary, flowId, status: 'running' } as FlowCardChatItem];
    });
  }

  public updateFlowCardStatus(flowId: string, status: 'running' | 'completed'): void {
    this.coreChat.update(history => history.map(item => item.type === 'flow_card' && item.flowId === flowId ? { ...item, status } : item));
  }

  public clearThinkingIndicators(): void {
    this.coreChat.update(chat => chat.filter(c => !(c.type === 'system' && c.content.includes('Thinking'))));
  }
}