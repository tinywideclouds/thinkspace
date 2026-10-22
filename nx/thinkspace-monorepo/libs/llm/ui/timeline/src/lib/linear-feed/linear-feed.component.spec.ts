import { ComponentFixture, TestBed } from '@angular/core/testing';
import { LinearFeedComponent } from './linear-feed.component';
import { TimelineFacade } from '../timeline.facade';
import { ChatConnectionService, ChatItem, FlowState } from '@org/llm-state-chat';
import { signal, WritableSignal } from '@angular/core';
import { describe, beforeEach, it, expect, vi, Mock } from 'vitest';
import { ComponentRef } from '@angular/core';

interface MockTimelineFacade {
  viewMode: WritableSignal<'linear' | 'cognitive'>;
  hoveredTag: WritableSignal<string | null>;
  selectedTag: WritableSignal<string | null>;
  isItemDimmed: Mock<(item: ChatItem) => boolean>;
  isItemCollapsed: Mock<(item: ChatItem) => boolean>;
  setHoveredTag: Mock<(tag: string | null) => void>;
  toggleSelectedTag: Mock<(tag: string) => void>;
  inspectFlow: Mock<(flowId: string) => void>;
}

interface MockConnectionService {
  submitPrompt: Mock<(text: string) => void>;
  loadHistoryPage: Mock<() => void>;
}

describe('LinearFeedComponent', () => {
  let component: LinearFeedComponent;
  let fixture: ComponentFixture<LinearFeedComponent>;
  let componentRef: ComponentRef<LinearFeedComponent>;
  let mockFacade: MockTimelineFacade;
  let mockConnection: MockConnectionService;

  beforeEach(async () => {
    mockFacade = {
      viewMode: signal('linear'),
      hoveredTag: signal<string | null>(null),
      selectedTag: signal<string | null>(null),
      isItemDimmed: vi.fn((item: ChatItem) => false),
      isItemCollapsed: vi.fn((item: ChatItem) => false),
      setHoveredTag: vi.fn(),
      toggleSelectedTag: vi.fn(),
      inspectFlow: vi.fn()
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
    componentRef = fixture.componentRef;

    componentRef.setInput('feed', [
      { id: '1', source: 'user', content: 'Test prompt' },
      { id: '2', source: 'model', content: 'Test response' },
      { id: '3', source: 'flow_card', content: 'Flow Card Test', flowId: 'f1' }
    ] as ChatItem[]);
    
    componentRef.setInput('flowStates', new Map<string, FlowState>([
      ['f1', { flowId: 'f1', taskId: 't1', agentCount: 1, completedCount: 0, status: 'running', agents: new Map() } as FlowState]
    ]));

    await fixture.whenStable();
    fixture.detectChanges();
  });

  it('should create the component', () => {
    expect(component).toBeTruthy();
  });

  it('should render user and model messages', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    
    // Validate text content renders correctly through the polymorphic blocks
    expect(compiled.textContent).toContain('Test prompt');
    expect(compiled.textContent).toContain('Test response');
  });
});