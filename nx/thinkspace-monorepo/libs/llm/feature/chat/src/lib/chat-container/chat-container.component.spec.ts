import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ChatContainerComponent } from './chat-container.component';
import { 
  ChatConnectionService, 
  FlowStore, 
  CognitiveStore, 
  WorkspaceStateService,
  FlowState,
  LogItem
} from '@org/llm-state-chat';
import { 
  DomainPreviewContext, 
  FileContextMetaData, 
  DomainAttentionFocus 
} from '@org/llm-core-facade';
import { TimelineFacade } from '@org/llm-ui-timeline';
import { signal, WritableSignal } from '@angular/core';
import { describe, beforeEach, it, expect, vi, Mock } from 'vitest';

// Mocks for Dumb UI Components
vi.mock('@org/llm-ui-timeline', () => ({
  LinearFeedComponent: { standalone: true, template: '<div id="mock-linear-feed"></div>' },
  TimelineFacade: vi.fn()
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

interface MockConnectionService {
  connected: WritableSignal<boolean>;
  clearSession: Mock<() => void>;
}

interface MockFlowStore {
  systemLogs: WritableSignal<LogItem[]>;
  flowStates: WritableSignal<Map<string, FlowState>>;
  retainLogs: WritableSignal<boolean>;
  inspectedReceipt: WritableSignal<string | null>;
  manualClearLogs: Mock<() => void>;
}

interface MockCognitiveStore {
  activeWorkbench: WritableSignal<FileContextMetaData[]>;
  pendingContextReview: WritableSignal<DomainPreviewContext | null>;
  pendingTransition: WritableSignal<{targetDomain: string, reasoning: string} | null>;
  pendingStrategy: WritableSignal<boolean>;
  pendingReviewBranch: WritableSignal<string | null>;
  copyFullContextPreview: Mock<(previewId: string) => Promise<boolean>>;
}

interface MockTimelineFacade {
  viewMode: WritableSignal<'linear' | 'cognitive'>;
  hoveredTag: WritableSignal<string | null>;
  activeFocus: WritableSignal<DomainAttentionFocus | null>;
}

interface MockWorkspaceState {
  spaces: WritableSignal<{ id: string; name: string; isConfigured: boolean }[]>;
  activeSpaceId: WritableSignal<string | null>;
  chats: WritableSignal<{ id: string; name: string; createdAt: string }[]>;
  activeChatId: WritableSignal<string | null>;
  loadChats: Mock<(spaceId: string) => void>;
  createChat: Mock<(name: string) => void>;
}

describe('ChatContainerComponent', () => {
  let component: ChatContainerComponent;
  let fixture: ComponentFixture<ChatContainerComponent>;
  
  let mockConnection: MockConnectionService;
  let mockFlow: MockFlowStore;
  let mockCognitive: MockCognitiveStore;
  let mockWorkspace: MockWorkspaceState;
  let mockTimelineFacade: MockTimelineFacade;

  beforeEach(async () => {
    mockConnection = { 
      connected: signal(true), 
      clearSession: vi.fn() 
    };
    
    mockFlow = { 
      systemLogs: signal<LogItem[]>([]), 
      flowStates: signal(new Map<string, FlowState>()), 
      retainLogs: signal(false), 
      inspectedReceipt: signal<string | null>(null), 
      manualClearLogs: vi.fn() 
    };
    
    mockCognitive = { 
      activeWorkbench: signal<FileContextMetaData[]>([]), 
      pendingContextReview: signal<DomainPreviewContext | null>(null),
      pendingTransition: signal<{targetDomain: string, reasoning: string} | null>(null), 
      pendingStrategy: signal(false),
      pendingReviewBranch: signal<string | null>(null), 
      copyFullContextPreview: vi.fn()
    };
    
    mockTimelineFacade = {
      viewMode: signal('linear'), 
      hoveredTag: signal<string | null>(null),
      activeFocus: signal<DomainAttentionFocus | null>({ primaryFocus: ['auth'], relatedContext: [], dormantLenses: [] })
    };
    
    mockWorkspace = {
      spaces: signal([{ id: 'space-1', name: 'Dev', isConfigured: true }]), 
      activeSpaceId: signal('space-1'),
      chats: signal([{ id: 'chat-1', name: 'Refactor', createdAt: '2026-10-21T00:00:00Z' }]), 
      activeChatId: signal('chat-1'),
      loadChats: vi.fn(), 
      createChat: vi.fn()
    };

    await TestBed.configureTestingModule({
      imports: [ChatContainerComponent],
      providers: [
        { provide: ChatConnectionService, useValue: mockConnection },
        { provide: FlowStore, useValue: mockFlow },
        { provide: CognitiveStore, useValue: mockCognitive },
        { provide: WorkspaceStateService, useValue: mockWorkspace },
        { provide: TimelineFacade, useValue: mockTimelineFacade }
      ]
    }).compileComponents();

    fixture = TestBed.createComponent(ChatContainerComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create the shell component', () => {
    expect(component).toBeTruthy();
  });

  it('should clear chat session when switching chats', () => {
    component.onChatSelected('chat-2');
    expect(mockWorkspace.activeChatId()).toBe('chat-2');
    expect(mockConnection.clearSession).toHaveBeenCalled();
  });
});