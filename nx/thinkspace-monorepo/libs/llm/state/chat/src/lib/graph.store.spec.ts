import { TestBed } from '@angular/core/testing';
import { GraphStore } from './graph.store';
import { ChatItem } from './chat.models';
import { ProjectionCoordinate } from '@org/llm-core-facade';
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

  it('should initialize with empty maps', () => {
    expect(store.nodes().size).toBe(0);
    expect(store.projections().size).toBe(0);
  });

  it('should upsert new nodes into the map', () => {
    const nodeA = { id: '1', type: 'prompt', content: 'A', timestamp: mockTimestamp } as unknown as ChatItem;
    const nodeB = { id: '2', type: 'model', content: 'B', timestamp: mockTimestamp } as unknown as ChatItem;

    store.upsert([nodeA, nodeB]);

    const map = store.nodes();
    expect(map.size).toBe(2);
    expect(map.get('1')).toEqual(nodeA);
  });

  it('should construct a valid key', () => {
    const coord: ProjectionCoordinate = { spaceId: 'golang', chatId: 'chat-1', projectionId: 'main' };
    expect(GraphStore.makeKey(coord)).toBe('golang:chat-1:main');
  });

  it('should reset projection pointers', () => {
    const coord: ProjectionCoordinate = { spaceId: 'golang', chatId: 'chat-1', projectionId: 'main' };
    store.resetProjection(coord, ['node-1', 'node-2']);

    const pointers = store.projections().get('golang:chat-1:main');
    expect(pointers).toEqual(['node-1', 'node-2']);
  });

  it('should append to projection pointers and deduplicate', () => {
    const coord: ProjectionCoordinate = { spaceId: 'golang', chatId: 'chat-1', projectionId: 'main' };
    store.resetProjection(coord, ['node-1']);
    store.appendToProjection(coord, ['node-2', 'node-1']);

    const pointers = store.projections().get('golang:chat-1:main');
    expect(pointers).toEqual(['node-1', 'node-2']);
  });
});