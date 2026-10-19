import { TestBed } from '@angular/core/testing';
import { CognitiveGraphStore } from './cognitive-graph.store';
import { ChatStateService } from '@org/llm-state-chat';
import { signal } from '@angular/core';
import { describe, beforeEach, it, expect, vi } from 'vitest';

describe('CognitiveGraphStore', () => {
  let store: CognitiveGraphStore;
  let mockChatState: any;

  beforeEach(() => {
    mockChatState = {
      activeDigests: signal({
        'node-1': { id: 'node-1', summary: 'System rule #auth', isSticky: true },
        'node-2': { id: 'node-2', summary: 'Past trajectory #database', isSticky: false }
      }),
      coreChat: signal([]),
      flowStates: signal(new Map()),
      activeFocus: signal({
        primaryFocus: ['auth'],
        relatedContext: [],
        dormantLenses: []
      }),
      inspectFlow: vi.fn()
    };

    TestBed.configureTestingModule({
      providers: [
        CognitiveGraphStore,
        { provide: ChatStateService, useValue: mockChatState }
      ]
    });

    store = TestBed.inject(CognitiveGraphStore);
  });

  it('should accurately partition sticky nodes from trajectory nodes', () => {
    expect(store.stickyNodes().length).toBe(1);
    expect(store.stickyNodes()[0].id).toBe('node-1');

    expect(store.trajectoryNodes().length).toBe(1);
    expect(store.trajectoryNodes()[0].id).toBe('node-2');
  });

  it('should detect if a node summary contains an active focus tag', () => {
    expect(store.isNodeInFocus('System rule #auth')).toBe(true);
    expect(store.isNodeInFocus('Past trajectory #database')).toBe(false);
  });

  it('should delegate inspectFlow to the chat state service', () => {
    store.inspectFlow('flow-xyz');
    expect(mockChatState.inspectFlow).toHaveBeenCalledWith('flow-xyz');
  });
});