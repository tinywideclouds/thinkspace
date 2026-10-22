import { TestBed } from '@angular/core/testing';
import { CognitiveGraphFacade } from './cognitive-graph.facade';
import { CognitiveStore, TimelineStore, FlowStore, ChatItem, FlowState } from '@org/llm-state-chat';
import { signal, WritableSignal } from '@angular/core';
import { describe, beforeEach, it, expect, vi, Mock } from 'vitest';
import { DomainAttentionFocus, DomainDigestMeta } from '@org/llm-core-facade';

interface MockCognitiveStore {
  activeDigests: WritableSignal<Record<string, DomainDigestMeta>>;
  activeFocus: WritableSignal<DomainAttentionFocus>;
}

interface MockTimelineStore {
  coreChat: WritableSignal<ChatItem[]>;
}

interface MockFlowStore {
  flowStates: WritableSignal<Map<string, FlowState>>;
  inspectFlow: Mock<(flowId: string) => void>;
}

describe('CognitiveGraphFacade', () => {
  let facade: CognitiveGraphFacade;
  let mockCognitive: MockCognitiveStore;
  let mockTimeline: MockTimelineStore;
  let mockFlow: MockFlowStore;

  beforeEach(() => {
    mockCognitive = {
      activeDigests: signal({
        'node-1': { id: 'node-1', summary: 'System rule #auth', isSticky: true },
        'node-2': { id: 'node-2', summary: 'Past trajectory #database', isSticky: false }
      }),
      activeFocus: signal({ primaryFocus: ['auth'], relatedContext: [], dormantLenses: [] })
    };
    
    mockTimeline = { coreChat: signal([]) };
    mockFlow = { flowStates: signal(new Map<string, FlowState>()), inspectFlow: vi.fn() };

    TestBed.configureTestingModule({
      providers: [
        CognitiveGraphFacade,
        { provide: CognitiveStore, useValue: mockCognitive },
        { provide: TimelineStore, useValue: mockTimeline },
        { provide: FlowStore, useValue: mockFlow }
      ]
    });

    facade = TestBed.inject(CognitiveGraphFacade);
  });

  it('should accurately partition sticky nodes from trajectory nodes', () => {
    expect(facade.stickyNodes().length).toBe(1);
    expect(facade.stickyNodes()[0].id).toBe('node-1');

    expect(facade.trajectoryNodes().length).toBe(1);
    expect(facade.trajectoryNodes()[0].id).toBe('node-2');
  });

  it('should detect if a node summary contains an active focus tag', () => {
    expect(facade.isNodeInFocus('System rule #auth')).toBe(true);
    expect(facade.isNodeInFocus('Past trajectory #database')).toBe(false);
  });

  it('should delegate inspectFlow to the flow store', () => {
    facade.inspectFlow('flow-xyz');
    expect(mockFlow.inspectFlow).toHaveBeenCalledWith('flow-xyz');
  });
});