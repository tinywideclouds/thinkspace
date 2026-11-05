import { TestBed } from '@angular/core/testing';
import { ChatConnectionService } from './chat-connection.service';
import { TransportService } from '@org/llm-core-transport';
import { TimelineStore } from './timeline.store';
import { FlowStore } from './flow.store';
import { CognitiveStore } from './cognitive.store';
import { ThinkspaceStateService } from './thinkspace-state.service';
import { UiSettingsService } from './ui-settings.service';
import { Subject } from 'rxjs';
import { AppEvent, PreviewContext, AttentionFocus, DigestMeta, FileContextMetaData } from '@org/llm-core-facade';
import { WSEvent } from '@org/llm-core-protos';
import { describe, beforeEach, afterEach, it, expect, vi, Mock } from 'vitest';
import { signal, computed, Signal, WritableSignal } from '@angular/core';
import { ChatItem } from './chat.models';

interface MockTransportService {
  connect: Mock<(url: string) => Subject<WSEvent>>;
  disconnect: Mock<() => void>;
  send: Mock<(msg: WSEvent) => void>;
}

interface MockTimelineStore {
  appendCoreChat: Mock<(type: 'prompt' | 'model' | 'system', content: string) => void>;
  clearSession: Mock<() => void>;
  hasMoreHistory: WritableSignal<boolean>;
  isLoadingHistory: WritableSignal<boolean>;
  nextCursor: WritableSignal<string>;
  clearThinkingIndicators: Mock<() => void>;
  processSyncHistory: Mock<() => void>;
  ingestEvent: Mock<() => void>;
  appendToLastModelMessage: Mock<() => void>;
  nodeMap: WritableSignal<Map<string, ChatItem>>;
  coreChat: Signal<ChatItem[]>;
}

interface MockThinkspaceStateService {
  activeSpaceId: WritableSignal<string | null>;
  activeChatId: WritableSignal<string | null>;
}

interface MockFlowStore {
  resetTurn: Mock<() => void>;
  clearSession: Mock<() => void>;
  appendSystemLog: Mock<(level: string, msg: string) => void>;
  processTelemetry: Mock<(event: AppEvent) => void>;
}

interface MockCognitiveStore {
  resetTurn: Mock<() => void>;
  clearSession: Mock<() => void>;
  pendingContextReview: WritableSignal<PreviewContext | null>;
  pendingStrategy: WritableSignal<boolean>;
  pendingReviewBranch: WritableSignal<string | null>;
  pendingTransition: WritableSignal<{ targetThinkState: string; reasoning: string } | null>;
  activeFocus: WritableSignal<AttentionFocus | null>;
  activeDigests: WritableSignal<Record<string, DigestMeta>>;
  activeWorkbench: WritableSignal<FileContextMetaData[]>;
  autoApproveContext: WritableSignal<boolean>;
}

describe('ChatConnectionService', () => {
  let service: ChatConnectionService;
  let transportMock: MockTransportService;
  let timelineMock: MockTimelineStore;
  let workspaceMock: MockThinkspaceStateService;
  let flowMock: MockFlowStore;
  let cognitiveMock: MockCognitiveStore;
  let wsSubject: Subject<WSEvent>;

  beforeEach(() => {
    vi.stubGlobal('Temporal', {
      Now: { instant: () => ({ toString: () => '2026-10-23T12:00:00Z' }) },
      Instant: { from: (s: string) => ({ toString: () => s }) }
    });

    wsSubject = new Subject<WSEvent>();

    transportMock = {
      connect: vi.fn(() => wsSubject),
      disconnect: vi.fn(),
      send: vi.fn()
    };

    const nodeMapSignal = signal(new Map<string, ChatItem>());

    timelineMock = {
      appendCoreChat: vi.fn(),
      clearSession: vi.fn(),
      hasMoreHistory: signal(false),
      isLoadingHistory: signal(false),
      nextCursor: signal(''),
      clearThinkingIndicators: vi.fn(),
      processSyncHistory: vi.fn(),
      ingestEvent: vi.fn(),
      appendToLastModelMessage: vi.fn(),
      nodeMap: nodeMapSignal,
      coreChat: computed(() => Array.from(nodeMapSignal().values()))
    };

    workspaceMock = {
      activeSpaceId: signal('space-1'),
      activeChatId: signal('chat-1')
    };

    flowMock = {
      resetTurn: vi.fn(),
      clearSession: vi.fn(),
      appendSystemLog: vi.fn(),
      processTelemetry: vi.fn()
    };

    cognitiveMock = {
      resetTurn: vi.fn(),
      clearSession: vi.fn(),
      pendingContextReview: signal(null),
      pendingStrategy: signal(false),
      pendingReviewBranch: signal('branch-1'),
      pendingTransition: signal(null),
      activeFocus: signal(null),
      activeDigests: signal({}),
      activeWorkbench: signal([]),
      autoApproveContext: signal(false)
    };

    TestBed.configureTestingModule({
      providers: [
        ChatConnectionService,
        { provide: TransportService, useValue: transportMock },
        { provide: TimelineStore, useValue: timelineMock },
        { provide: ThinkspaceStateService, useValue: workspaceMock },
        { provide: FlowStore, useValue: flowMock },
        { provide: CognitiveStore, useValue: cognitiveMock },
        { provide: UiSettingsService, useValue: { pagination: signal({ size: 50 }) } }
      ]
    });
    
    service = TestBed.inject(ChatConnectionService);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('should format switchBranch correctly and clear the normalized timeline map', () => {
    timelineMock.nodeMap().set('msg-1', { id: 'msg-1', type: 'prompt', content: 'test' } as ChatItem);
    expect(timelineMock.nodeMap().size).toBe(1);

    service.switchBranch('feature-v2');
    
    expect(timelineMock.nodeMap().size).toBe(0);
    expect(timelineMock.coreChat()).toEqual([]);
    expect(timelineMock.isLoadingHistory()).toBe(true);
    expect(transportMock.send).toHaveBeenCalled();
  });

  it('should handle request_transition mapping to targetThinkState', () => {
    service['routeIncomingEvent']({
      type: 'request_transition',
      targetThinkState: 'golang',
      reasoning: 'test'
    });
    
    expect(cognitiveMock.pendingTransition()).toEqual({ targetThinkState: 'golang', reasoning: 'test' });
  });
});