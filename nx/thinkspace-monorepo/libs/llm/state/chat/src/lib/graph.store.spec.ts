import { TestBed } from '@angular/core/testing';
import { GraphStore } from './graph.store';
import { ChatItem } from './chat.models';
import { describe, beforeEach, it, expect } from 'vitest';

describe('GraphStore', () => {
  let store: GraphStore;

  const mockTimestamp = { toString: () => '2026-10-23T12:00:00Z' } as any;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [GraphStore]
    });
    store = TestBed.inject(GraphStore);
  });

  it('should initialize with an empty map', () => {
    expect(store.nodes().size).toBe(0);
  });

  it('should upsert new nodes into the map', () => {
    const nodeA = { id: '1', type: 'prompt', content: 'A', timestamp: mockTimestamp } as unknown as ChatItem;
    const nodeB = { id: '2', type: 'model', content: 'B', timestamp: mockTimestamp } as unknown as ChatItem;

    store.upsert([nodeA, nodeB]);

    const map = store.nodes();
    expect(map.size).toBe(2);
    expect(map.get('1')).toEqual(nodeA);
    expect(map.get('2')).toEqual(nodeB);
  });

  it('should overwrite existing nodes on upsert', () => {
    const nodeA1 = { id: '1', type: 'prompt', content: 'Original', timestamp: mockTimestamp } as unknown as ChatItem;
    const nodeA2 = { id: '1', type: 'prompt', content: 'Updated', tombstoned: true, timestamp: mockTimestamp } as unknown as ChatItem;

    store.upsert([nodeA1]);
    expect(store.nodes().get('1')?.content).toBe('Original');

    store.upsert([nodeA2]);
    expect(store.nodes().get('1')?.content).toBe('Updated');
    expect(store.nodes().get('1')?.tombstoned).toBe(true);
  });

  it('should clear all nodes', () => {
    store.upsert([{ id: '1', type: 'prompt', content: 'A', timestamp: mockTimestamp } as unknown as ChatItem]);
    expect(store.nodes().size).toBe(1);

    store.clear();
    expect(store.nodes().size).toBe(0);
  });
});