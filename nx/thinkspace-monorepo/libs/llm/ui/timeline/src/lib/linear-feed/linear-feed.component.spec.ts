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
      workingMemoryFeed: signal([
        { id: '1', type: 'user', content: 'Test prompt' },
        { id: '2', type: 'model', content: 'Test response' },
        { id: '3', type: 'flow_card', content: 'Flow Card Test', flowId: 'f1', status: 'running' }
      ] as ChatItem[]),
      flowStates: signal(new Map<string, FlowState>([
        ['f1', { flowId: 'f1', taskId: 't1', agentCount: 1, completedCount: 0, status: 'running', agents: new Map() } as FlowState]
      ])),
      isItemDimmed: vi.fn((item: ChatItem) => false),
      isItemCollapsed: vi.fn((item: ChatItem) => false),
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

  it('should correctly identify the active user prompt', () => {
    expect(component.activeUserPromptId()).toBe('1');
  });
});