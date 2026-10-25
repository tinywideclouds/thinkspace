import { TestBed } from '@angular/core/testing';
import { TimelineFacade } from './timeline.facade';
import { TimelineStore, FlowStore, CognitiveStore, WorkspaceStateService, ChatItem, FlowState } from '@org/llm-state-chat';
import { signal, WritableSignal } from '@angular/core';
import { describe, beforeEach, it, expect, vi, Mock } from 'vitest';
import { DomainAttentionFocus, DomainDigestMeta } from '@org/llm-core-facade';

interface MockTimelineStore {
  coreChat: WritableSignal<ChatItem[]>;
}

interface MockFlowStore {
  flowStates: WritableSignal<Map<string, FlowState>>;
  inspectFlow: Mock<(flowId: string) => void>;
}

interface MockCognitiveStore {
  activeDigests: WritableSignal<Record<string, DomainDigestMeta>>;
  activeFocus: WritableSignal<DomainAttentionFocus | null>;
}

interface MockWorkspaceStateService {
  getFileContent: Mock<(path: string, ref: string) => Promise<string>>;
}

describe('TimelineFacade', () => {
  let facade: TimelineFacade;
  let mockTimeline: MockTimelineStore;
  let mockFlow: MockFlowStore;
  let mockCognitive: MockCognitiveStore;
  let mockWorkspace: MockWorkspaceStateService;

  beforeEach(() => {
    mockTimeline = {
      coreChat: signal([])
    };

    mockFlow = {
      flowStates: signal(new Map()),
      inspectFlow: vi.fn()
    };

    mockCognitive = {
      activeDigests: signal({}),
      activeFocus: signal(null)
    };

    mockWorkspace = {
      getFileContent: vi.fn().mockResolvedValue('Mock file content')
    };

    TestBed.configureTestingModule({
      providers: [
        TimelineFacade,
        { provide: TimelineStore, useValue: mockTimeline },
        { provide: FlowStore, useValue: mockFlow },
        { provide: CognitiveStore, useValue: mockCognitive },
        { provide: WorkspaceStateService, useValue: mockWorkspace }
      ]
    });

    facade = TestBed.inject(TimelineFacade);
  });

  it('should be created', () => {
    expect(facade).toBeTruthy();
  });

  it('should bundle items with parent_id into attachments array of the parent', () => {
    mockTimeline.coreChat.set([
      { id: 'parent-1', type: 'model', content: 'Here is a file' },
      { id: 'child-1', type: 'file_delta', content: 'delta', metadata: { parent_id: 'parent-1' } } as any
    ]);
    
    const feed = facade.enhancedFeed();
    
    expect(feed.length).toBe(1); // The child should not be in the root feed
    expect(feed[0].id).toBe('parent-1');
    expect((feed[0] as any).attachments.length).toBe(1);
    expect((feed[0] as any).attachments[0].id).toBe('child-1');
  });

  it('should toggle view mode', () => {
    expect(facade.viewMode()).toBe('linear');
    facade.toggleViewMode();
    expect(facade.viewMode()).toBe('cognitive');
  });

  it('should toggle debug mode', () => {
    expect(facade.isDebugMode()).toBe(true);
    facade.toggleDebugMode();
    expect(facade.isDebugMode()).toBe(false);
  });

  it('should delegate getFileContent to the workspace service', async () => {
    const content = await facade.getFileContent('src/main.ts', 'ref123');
    expect(content).toBe('Mock file content');
    expect(mockWorkspace.getFileContent).toHaveBeenCalledWith('src/main.ts', 'ref123');
  });
});