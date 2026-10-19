import { TestBed } from '@angular/core/testing';
import { LinearFeedStore } from './linear-feed.store';
import { ChatStateService } from '@org/llm-state-chat';
import { signal } from '@angular/core';
import { describe, beforeEach, it, expect, vi } from 'vitest';

describe('LinearFeedStore', () => {
  let store: LinearFeedStore;
  let mockChatState: any;

  beforeEach(() => {
    mockChatState = {
      coreChat: signal([
        { id: 'msg-1', source: 'user', content: 'Hello' },
        { id: 'msg-2', source: 'model', content: 'World' }
      ]),
      flowStates: signal(new Map([['flow-1', { status: 'running' }]])),
      inspectFlow: vi.fn()
    };

    TestBed.configureTestingModule({
      providers: [
        LinearFeedStore,
        { provide: ChatStateService, useValue: mockChatState }
      ]
    });

    store = TestBed.inject(LinearFeedStore);
  });

  it('should initialize and pass through coreChat and flowStates', () => {
    expect(store.feed().length).toBe(2);
    expect(store.flowStates().has('flow-1')).toBe(true);
  });

  it('should compute visibleFeed correctly when no filter is active', () => {
    store.activeTagFilter.set(null);
    expect(store.visibleFeed().length).toBe(2);
  });

  it('should delegate inspectFlow to the global chat state service', () => {
    store.inspectFlow('flow-123');
    expect(mockChatState.inspectFlow).toHaveBeenCalledWith('flow-123');
  });
});