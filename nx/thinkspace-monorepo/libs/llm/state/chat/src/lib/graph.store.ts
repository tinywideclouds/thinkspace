import { Injectable, signal } from '@angular/core';
import { ChatItem } from './chat.models';

@Injectable({ providedIn: 'root' })
export class GraphStore {
  public nodes = signal(new Map());

  public upsert(events: ChatItem[]): void {
    this.nodes.update(map => {
      const next = new Map(map);
      for (const ev of events) {
        next.set(ev.id, ev);
      }
      return next;
    });
  }

  public clear(): void {
    this.nodes.set(new Map());
  }
}