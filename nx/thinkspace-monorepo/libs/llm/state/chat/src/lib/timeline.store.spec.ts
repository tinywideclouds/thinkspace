import { TestBed } from '@angular/core/testing';
import { TimelineStore } from './timeline.store';
import { GraphStore } from './graph.store';
import { ChatItem } from './chat.models';
import { describe, beforeEach, it, expect } from 'vitest';

describe('TimelineStore', () => {
  let store: TimelineStore;
  let graph: GraphStore;

  const mockTime = (timeStr: string) => ({ toString: () => timeStr } as any);

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [TimelineStore, GraphStore]
    });
    store = TestBed.inject(TimelineStore);
    graph = TestBed.inject(GraphStore);
  });

  it('should project only active branch nodes from the GraphStore', () => {
    store.setHistoryMetadata('', false, 'main', {});

    graph.upsert([
      { id: '1', type: 'prompt', content: 'test', branchId: 'main', timestamp: mockTime('2026-10-23T10:00:00Z') } as unknown as ChatItem,
      { id: '2', type: 'prompt', content: 'test', branchId: 'feature', timestamp: mockTime('2026-10-23T11:00:00Z') } as unknown as ChatItem,
      { id: '3', type: 'prompt', content: 'test', branchId: 'main', timestamp: mockTime('2026-10-23T12:00:00Z') } as unknown as ChatItem
    ]);

    const projected = store.coreChat();
    expect(projected.length).toBe(2);
    expect(projected[0].id).toBe('1');
    expect(projected[1].id).toBe('3');
  });

  it('should filter out tombstoned nodes', () => {
    store.setHistoryMetadata('', false, 'main', {});

    graph.upsert([
      { id: '1', type: 'prompt', content: 'test', branchId: 'main', timestamp: mockTime('2026-10-23T10:00:00Z'), tombstoned: false } as unknown as ChatItem,
      { id: '2', type: 'prompt', content: 'test', branchId: 'main', timestamp: mockTime('2026-10-23T11:00:00Z'), tombstoned: true } as unknown as ChatItem
    ]);

    const projected = store.coreChat();
    expect(projected.length).toBe(1);
    expect(projected[0].id).toBe('1');
  });

  it('should project nodes sorted chronologically by timestamp', () => {
    store.setHistoryMetadata('', false, 'main', {});

    graph.upsert([
      { id: '3', type: 'prompt', content: 'test', branchId: 'main', timestamp: mockTime('2026-10-23T12:00:00Z') } as unknown as ChatItem,
      { id: '1', type: 'prompt', content: 'test', branchId: 'main', timestamp: mockTime('2026-10-23T10:00:00Z') } as unknown as ChatItem,
      { id: '2', type: 'prompt', content: 'test', branchId: 'main', timestamp: mockTime('2026-10-23T11:00:00Z') } as unknown as ChatItem
    ]);

    const projected = store.coreChat();
    expect(projected.length).toBe(3);
    expect(projected[0].id).toBe('1');
    expect(projected[1].id).toBe('2');
    expect(projected[2].id).toBe('3');
  });
});