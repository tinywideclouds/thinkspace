import { TestBed } from '@angular/core/testing';
import { TimelineFacade } from './timeline.facade';
import { TimelineStore, FlowStore, CognitiveStore, ThinkspaceStateService, ChatItem, FlowState, UiSettingsService, FlowExecutionChatItem } from '@org/llm-state-chat';
import { signal, WritableSignal } from '@angular/core';
import { describe, beforeEach, it, expect, vi, Mock } from 'vitest';
import { AttentionFocus, DigestMeta } from '@org/llm-core-facade';

interface MockTimelineStore {
  coreChat: WritableSignal<ChatItem[]>;
  isLoadingHistory: WritableSignal<boolean>;
}

interface MockFlowStore {
  flowStates: WritableSignal<Map<string, FlowState>>;
  inspectFlow: Mock<(item: FlowExecutionChatItem) => void>;
}

interface MockCognitiveStore {
  activeDigests: WritableSignal<Record<string, DigestMeta>>;
  activeFocus: WritableSignal<AttentionFocus | null>;
}

interface MockThinkspaceStateService {
  getFileContent: Mock<(path: string, ref: string) => Promise<string>>;
}

describe('TimelineFacade', () => {
  let facade: TimelineFacade;
  let mockTimeline: MockTimelineStore;
  let mockFlow: MockFlowStore;
  let mockCognitive: MockCognitiveStore;
  let mockWorkspace: MockThinkspaceStateService;
  let mockUiSettings: UiSettingsService;

  beforeEach(() => {
    mockTimeline = {
      coreChat: signal([]),
      isLoadingHistory: signal(false)
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
        UiSettingsService,
        { provide: TimelineStore, useValue: mockTimeline },
        { provide: FlowStore, useValue: mockFlow },
        { provide: CognitiveStore, useValue: mockCognitive },
        { provide: ThinkspaceStateService, useValue: mockWorkspace }
      ]
    });

    facade = TestBed.inject(TimelineFacade);
    mockUiSettings = TestBed.inject(UiSettingsService);
  });

  it('should bundle flat arrays into Turn objects', () => {
    const timestamp = Temporal.Now.instant();
    mockTimeline.coreChat.set([
      { id: '1', type: 'prompt', content: 'hello', timestamp } as ChatItem,
      { id: '2', type: 'model', content: 'hi', tags: ['greeting'], timestamp } as ChatItem,
      { id: '3', type: 'prompt', content: 'next', timestamp } as ChatItem
    ]);
    
    const feed = facade.visibleFeed();
    expect(feed.length).toBe(2);
    expect(feed[0].leader.id).toBe('1');
    expect(feed[0].blocks[0].id).toBe('2');
    expect(feed[0].tags).toContain('greeting');
    expect(feed[1].leader.id).toBe('3');
  });

  it('should apply visual tag filtering correctly', () => {
    const timestamp = Temporal.Now.instant();
    mockTimeline.coreChat.set([
      { id: '1', type: 'prompt', content: 'auth logic', timestamp } as ChatItem,
      { id: '2', type: 'model', content: 'auth reply', tags: ['auth'], timestamp } as ChatItem,
      { id: '3', type: 'prompt', content: 'db logic', timestamp } as ChatItem,
      { id: '4', type: 'model', content: 'db reply', tags: ['database'], timestamp } as ChatItem
    ]);
    
    mockUiSettings.tagFilterMode.set('visual');
    facade.toggleTagFilter('auth');
    
    const feed = facade.visibleFeed();
    expect(feed.length).toBe(1);
    expect(feed[0].tags).toContain('auth');
  });

  it('should dispatch focus when toggling a tag in context mode', () => {
    mockUiSettings.tagFilterMode.set('context');
    facade.toggleTagFilter('architecture');
    
    expect(mockCognitive.activeFocus()?.primaryFocus).toContain('architecture');
  });

  it('should clear selected turn IDs when exiting edit mode', () => {
    facade.toggleEditMode();
    facade.toggleTurnSelection('turn-1');
    expect(facade.selectedTurnIds().size).toBe(1);

    facade.toggleEditMode();
    expect(facade.selectedTurnIds().size).toBe(0);
  });
});