import { ComponentFixture, TestBed } from '@angular/core/testing';
import { LinearFeedComponent } from './linear-feed.component';
import { TimelineFacade } from '../timeline.facade';
import { ChatConnectionService, ChatItem, FlowState } from '@org/llm-state-chat';
import { signal, WritableSignal } from '@angular/core';
import { describe, beforeEach, it, expect, vi, Mock } from 'vitest';

interface MockTimelineFacade {
  viewMode: WritableSignal<'linear' | 'cognitive'>;
  isDebugMode: WritableSignal<boolean>;
  hoveredTag: WritableSignal<string | null>;
  selectedTag: WritableSignal<string | null>;
  workingMemoryFeed: WritableSignal<ChatItem[]>;
  flowStates: WritableSignal<Map<string, FlowState>>;
  isLoadingHistory: WritableSignal<boolean>;
  isItemDimmed: Mock<(item: ChatItem) => boolean>;
  isItemCollapsed: Mock<(item: ChatItem) => boolean>;
  setHoveredTag: Mock<(tag: string | null) => void>;
  toggleSelectedTag: Mock<(tag: string) => void>;
  inspectFlow: Mock<(flowId: string) => void>;
  toggleDebugMode: Mock<() => void>;
}

interface MockConnectionService {
  submitPrompt: Mock<(text: string) => void>;
  loadHistoryPage: Mock<() => void>;
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
      hoveredTag: signal<string | null>(null),
      selectedTag: signal<string | null>(null),
      workingMemoryFeed: signal([{ id: '1', type: 'model', content: 'test' } as ChatItem]),
      flowStates: signal(new Map()),
      isLoadingHistory: signal(false),
      isItemDimmed: vi.fn(() => false),
      isItemCollapsed: vi.fn(() => false),
      setHoveredTag: vi.fn(),
      toggleSelectedTag: vi.fn(),
      inspectFlow: vi.fn(),
      toggleDebugMode: vi.fn()
    };

    mockConnection = {
      submitPrompt: vi.fn(),
      loadHistoryPage: vi.fn()
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

    mockFacade.workingMemoryFeed.set([
      { id: '1', type: 'model', content: 'test' } as ChatItem,
      { id: '2', type: 'model', content: 'new message' } as ChatItem
    ]);
    
    fixture.detectChanges();
    await fixture.whenStable();

    expect(component.hasNewMessages()).toBe(true);
    
    const button = fixture.nativeElement.querySelector('button.absolute.bottom-6');
    expect(button.textContent.trim()).toContain('New Messages');
    expect(button.className).toContain('bg-blue-600');
  });

  it('should display the loading spinner when history is being fetched', () => {
    mockFacade.isLoadingHistory.set(true);
    fixture.detectChanges();

    const loaderText = fixture.nativeElement.textContent;
    expect(loaderText).toContain('Loading History...');
  });
});