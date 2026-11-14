import { ComponentFixture, TestBed } from '@angular/core/testing';
import { LinearFeedComponent } from './linear-feed.component';
import { TimelineFacade, Turn } from '../timeline.facade';
import { ChatConnectionService, ChatItem, FlowState, FlowExecutionChatItem } from '@org/llm-state-chat';
import { PreviewContext } from '@org/llm-core-facade';
import { signal, WritableSignal } from '@angular/core';
import { describe, beforeEach, it, expect, vi, Mock } from 'vitest';

interface MockTimelineFacade {
  viewMode: WritableSignal<'linear' | 'cognitive'>;
  isDebugMode: WritableSignal<boolean>;
  isEditMode: WritableSignal<boolean>;
  hoveredTag: WritableSignal<string | null>;
  selectedTag: WritableSignal<string | null>;
  selectedTurnIds: WritableSignal<Set<string>>;
  activeTagFilters: WritableSignal<Set<string>>;
  visibleFeed: WritableSignal<Turn[]>;
  flowStates: WritableSignal<Map<string, FlowState>>;
  isLoadingHistory: WritableSignal<boolean>;
  paginationSettings: WritableSignal<{ size: number, prefetchIndex: number }>;
  uiSettings: {
    tagFilterMode: WritableSignal<'visual' | 'context'>;
    bulkActionBarPosition: WritableSignal<'top' | 'bottom'>;
  };
  
  pendingContextReview: WritableSignal<PreviewContext | null>;
  pendingStrategy: WritableSignal<boolean>;
  pendingReviewBranch: WritableSignal<string | null>;
  pendingTransition: WritableSignal<{ targetThinkState: string; reasoning: string } | null>;
  inspectedFlow: WritableSignal<FlowExecutionChatItem | null>;

  isItemDimmed: Mock<(item: ChatItem) => boolean>;
  isItemCollapsed: Mock<(item: ChatItem) => boolean>;
  setHoveredTag: Mock<(tag: string | null) => void>;
  toggleSelectedTag: Mock<(tag: string) => void>;
  toggleTurnSelection: Mock<(id: string) => void>;
  inspectFlow: Mock<(flowId: string) => void>;
  copyFullContextPreview: Mock<(previewId: string) => void>;
  toggleDebugMode: Mock<() => void>;
  toggleEditMode: Mock<() => void>;
}

interface MockConnectionService {
  submitPrompt: Mock<(text: string) => void>;
  loadHistoryPage: Mock<() => void>;
  submitContextApproval: Mock<(approved: boolean, paths?: string[], indices?: number[]) => void>;
}

describe('LinearFeedComponent', () => {
  let component: LinearFeedComponent;
  let fixture: ComponentFixture<LinearFeedComponent>;
  let mockFacade: MockTimelineFacade;
  let mockConnection: MockConnectionService;

  beforeEach(async () => {
    mockFacade = {
      viewMode: signal('linear'),
      isDebugMode: signal(false),
      isEditMode: signal(false),
      hoveredTag: signal<string | null>(null),
      selectedTag: signal<string | null>(null),
      selectedTurnIds: signal(new Set<string>()),
      activeTagFilters: signal(new Set<string>()),
      visibleFeed: signal([
        { 
          id: 'turn-1', 
          leader: { 
            id: '1', 
            type: 'prompt', 
            content: 'test', 
            timestamp: Temporal.Now.instant() 
          } as ChatItem, 
          blocks: [], 
          tags: [] 
        }
      ]),
      flowStates: signal(new Map()),
      isLoadingHistory: signal(false),
      paginationSettings: signal({ size: 20, prefetchIndex: 4 }),
      uiSettings: {
        tagFilterMode: signal('visual'),
        bulkActionBarPosition: signal('top')
      },
      
      pendingContextReview: signal(null),
      pendingStrategy: signal(false),
      pendingReviewBranch: signal(null),
      pendingTransition: signal(null),
      inspectedFlow: signal(null),

      isItemDimmed: vi.fn(() => false),
      isItemCollapsed: vi.fn(() => false),
      setHoveredTag: vi.fn(),
      toggleSelectedTag: vi.fn(),
      toggleTurnSelection: vi.fn(),
      inspectFlow: vi.fn(),
      copyFullContextPreview: vi.fn(),
      toggleDebugMode: vi.fn(),
      toggleEditMode: vi.fn()
    };

    mockConnection = {
      submitPrompt: vi.fn(),
      loadHistoryPage: vi.fn(),
      submitContextApproval: vi.fn()
    };

    await TestBed.configureTestingModule({
      imports: [LinearFeedComponent],
      providers: [
        { provide: TimelineFacade, useValue: mockFacade },
        { provide: ChatConnectionService, useValue: mockConnection }
      ]
    }).compileComponents();

    fixture = TestBed.createComponent(LinearFeedComponent);
    component = fixture.componentInstance;

    await fixture.whenStable();
    fixture.detectChanges();
  });

  it('should create the component', () => {
    expect(component).toBeTruthy();
  });

  it('should not render the shell-scoped context inspector inside the linear feed', async () => {
    mockFacade.pendingContextReview.set({
      spaceId: 'golang',
      chatId: 'chat-1',
      managerSystemPrompt: 'System rules',
      stickyDigests: [],
      recentHistory: ['[user] hello'],
      fileContexts: [],
      focus: { primaryFocus: [], relatedContext: [], dormantLenses: [] }
    });

    fixture.detectChanges();
    await fixture.whenStable();

    const inspector = fixture.nativeElement.querySelector('llm-context-inspector');
    expect(inspector).toBeNull();
  });

  it('should display simple back-to-bottom arrow when in manual scroll state but no new messages exist', async () => {
    const viewport = component.viewport();
    if (viewport) {
      viewport.scrollState.set('manual');
    }
    
    fixture.detectChanges();
    await fixture.whenStable();

    expect(component.hasNewMessages()).toBe(false);
    
    const button = fixture.nativeElement.querySelector('button.absolute.bottom-6');
    expect(button.textContent.trim()).toBe('↓');
    expect(button.className).toContain('bg-white');
  });

  it('should upgrade to New Messages pill if feed length increases while in manual state', async () => {
    const viewport = component.viewport();
    
    if (viewport) viewport.scrollState.set('auto');
    fixture.detectChanges();
    
    if (viewport) viewport.scrollState.set('manual');
    fixture.detectChanges();

    mockFacade.visibleFeed.set([
      { 
        id: 'turn-1', 
        leader: { 
          id: '1', 
          type: 'prompt', 
          content: 'test', 
          timestamp: Temporal.Now.instant() 
        } as ChatItem, 
        blocks: [
          { 
            id: '2', 
            type: 'model', 
            content: 'new message', 
            timestamp: Temporal.Now.instant() 
          } as ChatItem
        ], 
        tags: [] 
      }
    ]);
    
    fixture.detectChanges();
    await fixture.whenStable();

    expect(component.hasNewMessages()).toBe(true);
    
    const button = fixture.nativeElement.querySelector('button.absolute.bottom-6');
    expect(button.textContent.trim()).toContain('New Messages');
    expect(button.className).toContain('bg-blue-600');
  });

  it('should display the loading skeleton when history is being fetched', () => {
    mockFacade.isLoadingHistory.set(true);
    fixture.detectChanges();

    const skeletonBlock = fixture.nativeElement.querySelector('llm-skeleton-block');
    expect(skeletonBlock).toBeTruthy();
  });

  it('should display the empty state message when visual filters yield no results', () => {
    mockFacade.activeTagFilters.set(new Set(['nonexistent']));
    mockFacade.visibleFeed.set([]);
    fixture.detectChanges();

    const emptyState = fixture.nativeElement.textContent;
    expect(emptyState).toContain('No context matches the active visual filters');
  });
});