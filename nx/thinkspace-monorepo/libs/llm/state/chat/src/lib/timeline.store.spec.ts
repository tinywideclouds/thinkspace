import { TestBed } from '@angular/core/testing';
import { TimelineStore } from './timeline.store';
import { GraphStore } from './graph.store';
import { ThinkspaceStateService } from './thinkspace-state.service';
import { ChatItem } from './chat.models';
import { signal, WritableSignal } from '@angular/core';
import { describe, beforeEach, it, expect, vi } from 'vitest';

describe('TimelineStore', () => {
  let store: TimelineStore;
  let graph: GraphStore;
  
  let workspaceMock: {
    activeSpaceId: WritableSignal<string>;
    activeChatId: WritableSignal<string>;
  };

  const mockTime = (timeStr: string) => ({ toString: () => timeStr } as any);

  beforeEach(() => {
    workspaceMock = {
      activeSpaceId: signal('space-1'),
      activeChatId: signal('chat-1')
    };

    TestBed.configureTestingModule({
      providers: [
        TimelineStore, 
        GraphStore,
        { provide: ThinkspaceStateService, useValue: workspaceMock }
      ]
    });
    store = TestBed.inject(TimelineStore);
    graph = TestBed.inject(GraphStore);
  });

  it('should project nodes via pointer lookup rather than filtering', () => {
    store.activeProjectionId.set('main');

    graph.upsert([
      { id: '1', type: 'prompt', content: 'A', timestamp: mockTime('2026-10-23T10:00:00Z') } as unknown as ChatItem,
      { id: '2', type: 'prompt', content: 'B', timestamp: mockTime('2026-10-23T11:00:00Z') } as unknown as ChatItem,
      { id: '3', type: 'prompt', content: 'C', timestamp: mockTime('2026-10-23T12:00:00Z') } as unknown as ChatItem
    ]);

    // Only assign 1 and 3 to 'main'
    graph.resetProjection({ spaceId: 'space-1', chatId: 'chat-1', projectionId: 'main' }, ['1', '3']);

    const projected = store.coreChat();
    expect(projected.length).toBe(2);
    expect(projected[0].id).toBe('1');
    expect(projected[1].id).toBe('3');
  });

  it('should filter out tombstoned nodes from the projection array', () => {
    store.activeProjectionId.set('main');

    graph.upsert([
      { id: '1', type: 'prompt', content: 'test', timestamp: mockTime('2026-10-23T10:00:00Z'), tombstoned: false } as unknown as ChatItem,
      { id: '2', type: 'prompt', content: 'test', timestamp: mockTime('2026-10-23T11:00:00Z'), tombstoned: true } as unknown as ChatItem
    ]);

    graph.resetProjection({ spaceId: 'space-1', chatId: 'chat-1', projectionId: 'main' }, ['1', '2']);

    const projected = store.coreChat();
    expect(projected.length).toBe(1);
    expect(projected[0].id).toBe('1');
  });

  it('should correctly evaluate isActiveView', () => {
    store.activeProjectionId.set('main');
    
    const active = store.isActiveView({ spaceId: 'space-1', chatId: 'chat-1', projectionId: 'main' });
    const inactive = store.isActiveView({ spaceId: 'space-2', chatId: 'chat-1', projectionId: 'main' });
    
    expect(active).toBe(true);
    expect(inactive).toBe(false);
  });
});