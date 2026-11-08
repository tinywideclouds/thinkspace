import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ComponentRef, signal, WritableSignal } from '@angular/core';
import { BlockRouterComponent } from './block-router.component';
import { TimelineFacade } from '../timeline.facade';
import { ChatItem, FlowState, FlowExecutionChatItem } from '@org/llm-state-chat';
import { describe, beforeEach, it, expect, vi, Mock } from 'vitest';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';

declare const Temporal: { Now: { instant: () => unknown } };

interface MockTimelineFacade {
  isItemDimmed: Mock<(item: ChatItem) => boolean>;
  isItemCollapsed: Mock<(item: ChatItem) => boolean>;
  hoveredTag: WritableSignal<string | null>;
  selectedTag: WritableSignal<string | null>;
  setHoveredTag: Mock<(tag: string | null) => void>;
  toggleSelectedTag: Mock<(tag: string) => void>;
  inspectFlow: Mock<(item: FlowExecutionChatItem) => void>;
  viewGraph: Mock<(flowId: string) => void>;
  flowStates: WritableSignal<Map<string, FlowState>>;
  isDebugMode: WritableSignal<boolean>;
  isEditMode: WritableSignal<boolean>;
  getFileContent: Mock<(path: string, ref: string) => Promise<string>>;
}

describe('BlockRouterComponent', () => {
  let component: BlockRouterComponent;
  let fixture: ComponentFixture<BlockRouterComponent>;
  let componentRef: ComponentRef<BlockRouterComponent>;
  let mockFacade: MockTimelineFacade;

  beforeEach(async () => {
    mockFacade = {
      isItemDimmed: vi.fn(() => false),
      isItemCollapsed: vi.fn(() => false),
      hoveredTag: signal(null),
      selectedTag: signal(null),
      setHoveredTag: vi.fn(),
      toggleSelectedTag: vi.fn(),
      inspectFlow: vi.fn(),
      viewGraph: vi.fn(),
      flowStates: signal(new Map<string, FlowState>()),
      isDebugMode: signal(false),
      isEditMode: signal(false),
      getFileContent: vi.fn().mockResolvedValue('')
    };

    await TestBed.configureTestingModule({
      imports: [BlockRouterComponent],
      providers: [
        { provide: TimelineFacade, useValue: mockFacade },
        provideHttpClient(),
        provideHttpClientTesting()
      ]
    }).compileComponents();

    fixture = TestBed.createComponent(BlockRouterComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;
  });

  it('should route to the user message block for prompt type', async () => {
    componentRef.setInput('chatItem', { 
      id: '1', 
      type: 'prompt', 
      content: 'User text',
      timestamp: Temporal.Now.instant()
    } as ChatItem);
    
    await fixture.whenStable();
    fixture.detectChanges();
    
    expect(fixture.nativeElement.querySelector('llm-user-message-block')).toBeTruthy();
  });

  it('should route to the model message block', async () => {
    componentRef.setInput('chatItem', { 
      id: '2', 
      type: 'model', 
      content: 'Model text',
      timestamp: Temporal.Now.instant()
    } as ChatItem);
    
    await fixture.whenStable();
    fixture.detectChanges();
    
    expect(fixture.nativeElement.querySelector('llm-model-message-block')).toBeTruthy();
  });

  it('should route to the file delta block', async () => {
    componentRef.setInput('chatItem', { 
      id: '3', 
      type: 'file_delta', 
      path: 'src/app.ts', 
      previewSnippet: 'patch', 
      fileRef: 'ref', 
      content: 'summary text',
      timestamp: Temporal.Now.instant()
    } as ChatItem);
    
    await fixture.whenStable();
    fixture.detectChanges();
    
    expect(fixture.nativeElement.querySelector('llm-file-delta-block')).toBeTruthy();
  });

  it('should route to the flow execution card component', async () => {
    componentRef.setInput('chatItem', { 
      id: '4', 
      type: 'flow_execution', 
      flowId: 'flow-123',
      taskId: 'task-123',
      task: 'Mock Task',
      status: 'RUNNING', 
      error: '',
      agents: [],
      content: 'Running agent tests',
      timestamp: Temporal.Now.instant()
    } as ChatItem);
    
    await fixture.whenStable();
    fixture.detectChanges();
    
    expect(fixture.nativeElement.querySelector('llm-chat-flow-card')).toBeTruthy();
  });
});