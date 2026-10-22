import { Injectable, signal } from '@angular/core';
import { DomainLedgerEvent } from '@org/llm-core-facade';
import { ChatItem } from './chat.models';

@Injectable({ providedIn: 'root' })
export class TimelineStore {
  public coreChat = signal<ChatItem[]>([]);
  public hasMoreHistory = signal<boolean>(false);
  public nextCursor = signal<string>('');
  public isLoadingHistory = signal<boolean>(false);

  public clearSession(): void {
    this.coreChat.set([]);
    this.hasMoreHistory.set(false);
    this.nextCursor.set('');
    this.isLoadingHistory.set(false);
  }

  public processSyncHistory(recentEvents: DomainLedgerEvent[], nextCursor: string, hasMore: boolean): void {
    const incomingTimeline: ChatItem[] = recentEvents
      .filter(e => e.type === 'prompt' || e.type === 'model')
      .map(e => ({ id: e.id, source: e.type === 'model' ? 'model' : 'user', content: e.content }));

    this.coreChat.update(existing => {
      if (this.isLoadingHistory() && existing.length > 0) {
        return [...incomingTimeline, ...existing];
      }
      return incomingTimeline;
    });

    this.nextCursor.set(nextCursor);
    this.hasMoreHistory.set(hasMore);
    this.isLoadingHistory.set(false);
  }

  public appendCoreChat(source: ChatItem['source'], content: string): void {
    this.coreChat.update(history => {
      const cleanHistory = history.filter(c => !(c.source === 'system' && c.content.includes('Thinking')));
      return [...cleanHistory, { id: crypto.randomUUID(), source, content }];
    });
  }

  public appendToLastModelMessage(chunkText: string): void {
    this.coreChat.update(history => {
      const cleanHistory = history.filter(c => !(c.source === 'system' && c.content.includes('Thinking')));
      if (cleanHistory.length === 0 || cleanHistory[cleanHistory.length - 1].source !== 'model') {
        return [...cleanHistory, { id: crypto.randomUUID(), source: 'model', content: chunkText }];
      }
      const updated = [...cleanHistory];
      updated[updated.length - 1].content += chunkText;
      return updated;
    });
  }

  public injectFlowCard(flowId: string, summary: string): void {
    this.coreChat.update(history => {
      const cleanHistory = history.filter(c => !(c.source === 'system' && c.content.includes('Thinking')));
      return [...cleanHistory, { id: crypto.randomUUID(), source: 'flow_card', content: summary, flowId, status: 'running' }];
    });
  }

  public updateFlowCardStatus(flowId: string, status: 'running' | 'completed'): void {
    this.coreChat.update(history => history.map(item => item.flowId === flowId ? { ...item, status } : item));
  }

  public clearThinkingIndicators(): void {
    this.coreChat.update(chat => chat.filter(c => !(c.source === 'system' && c.content.includes('Thinking'))));
  }
}