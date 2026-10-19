import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ChatContainerComponent } from './chat-container.component';
import { ChatStateService, WorkspaceStateService } from '@org/llm-state-chat';
import { signal } from '@angular/core';
import { describe, beforeEach, it, expect, vi } from 'vitest';

// Mocking child components to keep the test shallow and fast
vi.mock('@org/llm-ui-linear-feed', () => ({
  LinearFeedComponent: { standalone: true, template: '<div id="mock-linear-feed"></div>' }
}));
vi.mock('@org/llm-ui-cognitive-graph', () => ({
  CognitiveGraphComponent: { standalone: true, template: '<div id="mock-cognitive-graph"></div>' }
}));
vi.mock('@org/llm-ui-hud', () => ({
  FocusHudComponent: { standalone: true, template: '<div id="mock-focus-hud"></div>' }
}));
vi.mock('@org/llm-ui-workbench', () => ({
  DeskComponent: { standalone: true, template: '<div id="mock-desk"></div>' }
}));

describe('ChatContainerComponent', () => {
  let component: ChatContainerComponent;
  let fixture: ComponentFixture<ChatContainerComponent>;
  let mockChatState: any;
  let mockWorkspaceState: any;

  beforeEach(async () => {
    mockChatState = {
      connected: signal(true),
      autoApproveContext: signal(false),
      activeFocus: signal({ primaryFocus: ['auth'], relatedContext: [] }),
      activeWorkbench: signal([{ path: 'src/main.ts', level: 'structural', tokenCount: 150 }]),
      systemLogs: signal([]),
      flowStates: signal(new Map()),
      retainLogs: signal(false),
      pendingContextReview: signal(null),
      pendingTransition: signal(null),
      pendingStrategy: signal(null),
      pendingReviewBranch: signal(null),
      inspectedReceipt: signal(null),
      clearSession: vi.fn(),
      manualClearLogs: vi.fn()
    };

    mockWorkspaceState = {
      spaces: signal([{ id: 'space-1', name: 'Dev' }]),
      activeSpaceId: signal('space-1'),
      chats: signal([{ id: 'chat-1', name: 'Refactor' }]),
      activeChatId: signal('chat-1'),
      loadChats: vi.fn(),
      createChat: vi.fn()
    };

    await TestBed.configureTestingModule({
      imports: [ChatContainerComponent],
      providers: [
        { provide: ChatStateService, useValue: mockChatState },
        { provide: WorkspaceStateService, useValue: mockWorkspaceState }
      ]
    }).compileComponents();

    fixture = TestBed.createComponent(ChatContainerComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create the shell component', () => {
    expect(component).toBeTruthy();
  });

  it('should render the Focus HUD and Desk components', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('#mock-focus-hud')).toBeTruthy();
    expect(compiled.querySelector('#mock-desk')).toBeTruthy();
  });

  it('should default to linear view mode and render the linear feed', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    expect(component.viewMode()).toBe('linear');
    expect(compiled.querySelector('#mock-linear-feed')).toBeTruthy();
    expect(compiled.querySelector('#mock-cognitive-graph')).toBeFalsy();
  });

  it('should swap to cognitive graph when view mode is toggled', () => {
    component.viewMode.set('cognitive');
    fixture.detectChanges();

    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('#mock-linear-feed')).toBeFalsy();
    expect(compiled.querySelector('#mock-cognitive-graph')).toBeTruthy();
  });

  it('should clear chat session when switching chats', () => {
    component.onChatSelected('chat-2');
    expect(mockWorkspaceState.activeChatId()).toBe('chat-2');
    expect(mockChatState.clearSession).toHaveBeenCalled();
  });
});