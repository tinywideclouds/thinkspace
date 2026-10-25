import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ComponentRef, signal, WritableSignal } from '@angular/core';
import { BlockRouterComponent } from './block-router.component';
import { TimelineFacade } from '../timeline.facade';
import { ChatItem, FlowState } from '@org/llm-state-chat';
import { describe, beforeEach, it, expect, vi, Mock } from 'vitest';

interface MockTimelineFacade {
  isItemDimmed: Mock<(item: ChatItem) => boolean>;
  isItemCollapsed: Mock<(item: ChatItem) => boolean>;
  hoveredTag: WritableSignal<string | null>;
  selectedTag: WritableSignal<string | null>;
  setHoveredTag: Mock<(tag: string | null) => void>;
  toggleSelectedTag: Mock<(tag: string) => void>;
  inspectFlow: Mock<(flowId: string) => void>;
  flowStates: WritableSignal<Map<string, FlowState>>;
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
      flowStates: signal(new Map())
    };

    await TestBed.configureTestingModule({
      imports: [BlockRouterComponent],
      providers: [
        { provide: TimelineFacade, useValue: mockFacade }
      ]
    }).compileComponents();

    fixture = TestBed.createComponent(BlockRouterComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;
  });

  it('should route to the user message block', async () => {
    componentRef.setInput('chatItem', { id: '1', type: 'user', content: 'User text' });
    await fixture.whenStable();
    fixture.detectChanges();
    
    expect(fixture.nativeElement.querySelector('llm-user-message-block')).toBeTruthy();
  });

  it('should route to the model message block', async () => {
    componentRef.setInput('chatItem', { id: '2', type: 'model', content: 'Model text' });
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
      content: 'summary text' 
    });
    await fixture.whenStable();
    fixture.detectChanges();
    
    expect(fixture.nativeElement.querySelector('llm-file-delta-block')).toBeTruthy();
  });
});