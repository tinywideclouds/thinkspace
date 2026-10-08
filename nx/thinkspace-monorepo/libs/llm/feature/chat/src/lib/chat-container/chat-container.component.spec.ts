import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ChatContainerComponent } from './chat-container.component';
import { ChatStateService, WorkspaceStateService } from '@org/llm-state-chat';
import { signal } from '@angular/core';
import { describe, beforeEach, it, expect, vi } from 'vitest';

describe('ChatContainerComponent', () => {
  let component: ChatContainerComponent;
  let fixture: ComponentFixture<ChatContainerComponent>;
  let mockChatState: any;
  let mockWorkspaceState: any;

  beforeEach(async () => {
    mockChatState = {
      coreChat: signal([]),
      flowStates: signal(new Map()),
      systemLogs: signal([]),
      connected: signal(true),
      autoApproveContext: signal(true),
      retainLogs: signal(false),
      pendingContextReview: signal(null),
      pendingTransition: signal(null),
      pendingStrategy: signal(false),
      pendingReviewBranch: signal(null),
      inspectedReceipt: signal(null),
      submitPrompt: vi.fn(),
      submitContextApproval: vi.fn(),
      selectStrategy: vi.fn(),
      submitReview: vi.fn(),
      submitTransitionDecision: vi.fn(),
      clearSession: vi.fn(),
      manualClearLogs: vi.fn(),
      inspectFlow: vi.fn(),
      copyFullContextPreview: vi.fn(),
    };

    mockWorkspaceState = {
      spaces: signal([{ id: 'golang', name: 'Golang', isConfigured: true }]),
      activeSpaceId: signal('golang'),
      chats: signal([{ id: 'chat-1', name: 'Chat 1', createdAt: '2026-09-08T00:00:00Z' }]),
      activeChatId: signal('chat-1'),
      loadChats: vi.fn(),
      createChat: vi.fn(),
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

  it('should create the container component', () => {
    expect(component).toBeTruthy();
  });

  it('should render standard chat input when no pending actions exist', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('llm-chat-input')).toBeTruthy();
    expect(compiled.querySelector('llm-chat-transition-prompt')).toBeFalsy();
    expect(compiled.querySelector('llm-context-inspector')).toBeFalsy();
    expect(compiled.querySelector('llm-chat-strategy-prompt')).toBeFalsy();
    expect(compiled.querySelector('llm-chat-review-prompt')).toBeFalsy();
  });

  it('should render transition prompt when pendingTransition has a value', () => {
    mockChatState.pendingTransition.set({ targetDomain: 'golang', reasoning: 'need to compile' });
    fixture.detectChanges();

    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('llm-chat-transition-prompt')).toBeTruthy();
    expect(compiled.querySelector('llm-chat-input')).toBeFalsy();
  });

  it('should clear session and load chats when space is selected', () => {
    component.onSpaceSelected('concept');
    expect(mockWorkspaceState.loadChats).toHaveBeenCalledWith('concept');
    expect(mockChatState.clearSession).toHaveBeenCalled();
  });
  
  it('should clear session and set active chat when chat is selected', () => {
    component.onChatSelected('chat-2');
    expect(mockWorkspaceState.activeChatId()).toBe('chat-2');
    expect(mockChatState.clearSession).toHaveBeenCalled();
  });

  it('should request chat creation and clear session when new chat is requested', () => {
    // Stub prompt to simulate user input
    vi.stubGlobal('prompt', () => 'New Architecture Chat');
    
    component.onNewChatRequested();
    
    expect(mockWorkspaceState.createChat).toHaveBeenCalledWith('New Architecture Chat');
    expect(mockChatState.clearSession).toHaveBeenCalled();
    
    vi.unstubAllGlobals();
  });

  it('should not create a chat if the prompt is cancelled', () => {
    // Stub prompt to simulate user cancellation
    vi.stubGlobal('prompt', () => null);
    
    component.onNewChatRequested();
    
    expect(mockWorkspaceState.createChat).not.toHaveBeenCalled();
    expect(mockChatState.clearSession).not.toHaveBeenCalled();
    
    vi.unstubAllGlobals();
  });
});