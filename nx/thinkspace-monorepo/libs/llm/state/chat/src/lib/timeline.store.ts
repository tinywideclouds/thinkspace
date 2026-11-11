import { Injectable, computed, inject, signal } from '@angular/core';
import { GraphStore } from './graph.store';
import { ChatItem } from './chat.models';
import { ThinkspaceStateService } from './thinkspace-state.service';
import { ProjectionCoordinate } from '@org/llm-core-facade';

@Injectable({ providedIn: 'root' })
export class TimelineStore {
  private graph = inject(GraphStore);
  private workspace = inject(ThinkspaceStateService);

  public isLoadingHistory = signal<boolean>(false);
  public activeProjectionId = signal<string>('main');
  
  public nextCursor = signal<string>('');
  public hasMore = signal<boolean>(false);

  public coreChat = computed<ChatItem[]>(() => {
    const spaceId = this.workspace.activeSpaceId();
    const chatId = this.workspace.activeChatId();
    const projectionId = this.activeProjectionId();

    if (!spaceId || !chatId || !projectionId) return [];

    const key = GraphStore.makeKey({ spaceId, chatId, projectionId });
    const pointers = this.graph.projections().get(key) || [];
    const cache = this.graph.nodes();

    return pointers
      .map(id => cache.get(id))
      .filter((node): node is ChatItem => node !== undefined && !node.tombstoned);
  });

  public isActiveView(coordinate: ProjectionCoordinate): boolean {
    return (
      this.workspace.activeSpaceId() === coordinate.spaceId &&
      this.workspace.activeChatId() === coordinate.chatId &&
      this.activeProjectionId() === coordinate.projectionId
    );
  }

  public setHistoryMetadata(cursor: string, more: boolean, projectionId: string): void {
    this.nextCursor.set(cursor);
    this.hasMore.set(more);
    this.activeProjectionId.set(projectionId);
  }
}