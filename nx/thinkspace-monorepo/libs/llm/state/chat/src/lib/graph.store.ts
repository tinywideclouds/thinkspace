import { Injectable, signal } from '@angular/core';
import { ChatItem } from './chat.models';
import { ProjectionCoordinate } from '@org/llm-core-facade';

export type ProjectionKey = `${string}:${string}:${string}`;

@Injectable({ providedIn: 'root' })
export class GraphStore {
  public nodes = signal(new Map<string, ChatItem>());
  public projections = signal(new Map<ProjectionKey, string[]>());

  public static makeKey(c: ProjectionCoordinate): ProjectionKey {
    return `${c.spaceId}:${c.chatId}:${c.projectionId}`;
  }

  public upsert(events: ChatItem[]): void {
    this.nodes.update(map => {
      const next = new Map(map);
      for (const ev of events) {
        next.set(ev.id, ev);
      }
      return next;
    });
  }

  public appendToProjection(coordinate: ProjectionCoordinate, nodeIds: string[]): void {
    this.projections.update(map => {
      const next = new Map(map);
      const key = GraphStore.makeKey(coordinate);
      const current = next.get(key) || [];
      next.set(key, Array.from(new Set([...current, ...nodeIds])));
      return next;
    });
  }

  public resetProjection(coordinate: ProjectionCoordinate, nodeIds: string[]): void {
    this.projections.update(map => {
      const next = new Map(map);
      const key = GraphStore.makeKey(coordinate);
      next.set(key, nodeIds);
      return next;
    });
  }

  public clear(): void {
    this.nodes.set(new Map());
    this.projections.set(new Map());
  }
}