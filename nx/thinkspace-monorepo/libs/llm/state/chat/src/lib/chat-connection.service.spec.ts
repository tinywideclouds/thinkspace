import { TestBed } from '@angular/core/testing';
import { ChatConnectionService } from './chat-connection.service';
import { TransportService } from '@org/llm-core-transport';
import { GraphStore } from './graph.store';
import { TimelineStore } from './timeline.store';
import { FlowStore } from './flow.store';
import { CognitiveStore } from './cognitive.store';
import { ThinkspaceStateService } from './thinkspace-state.service';
import { UiSettingsService } from './ui-settings.service';
import { Subject } from 'rxjs';
import { AppEvent, ContextStage, PreviewContext, AttentionFocus, DigestMeta, FileContextMetaData } from '@org/llm-core-facade';
import { WSEvent, WSEventSchema } from '@org/llm-core-protos';
import { fromJson, JsonValue } from '@bufbuild/protobuf';
import { describe, beforeEach, afterEach, it, expect, vi, Mock } from 'vitest';
import { signal, computed, Signal, WritableSignal } from '@angular/core';
import { ChatItem } from './chat.models';

interface MockTransportService {
  connect: Mock<(url: string) => Subject<WSEvent>>;
  disconnect: Mock<() => void>;
  send: Mock<(msg: WSEvent) => void>;
}

interface MockTimelineStore {
  isLoadingHistory: WritableSignal<boolean>;
  activeProjectionId: WritableSignal<string>;
  nextCursor: WritableSignal<string>;
  hasMore: WritableSignal<boolean>;
  coreChat: Signal<ChatItem[]>;
  setHistoryMetadata: Mock<(cursor: string, more: boolean, branch: string) => void>;
  isActiveView: Mock<(c: any) => boolean>;
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
  manualClearLogs: Mock<() => void>;
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
  let graphStore: GraphStore;
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

    timelineMock = {
      isLoadingHistory: signal(false),
      activeProjectionId: signal('main'),
      nextCursor: signal(''),
      hasMore: signal(false),
      coreChat: computed(() => []),
      setHistoryMetadata: vi.fn(),
      isActiveView: vi.fn(() => true)
    };

    workspaceMock = {
      activeSpaceId: signal('space-1'),
      activeChatId: signal('chat-1')
    };

    flowMock = {
      resetTurn: vi.fn(),
      clearSession: vi.fn(),
      appendSystemLog: vi.fn(),
      processTelemetry: vi.fn(),
      manualClearLogs: vi.fn()
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
        GraphStore,
        { provide: TransportService, useValue: transportMock },
        { provide: TimelineStore, useValue: timelineMock },
        { provide: ThinkspaceStateService, useValue: workspaceMock },
        { provide: FlowStore, useValue: flowMock },
        { provide: CognitiveStore, useValue: cognitiveMock },
        { provide: UiSettingsService, useValue: { pagination: signal({ size: 50 }) } }
      ]
    });
    
    service = TestBed.inject(ChatConnectionService);
    graphStore = TestBed.inject(GraphStore);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('should automatically request initial loadHistory when connected with an active space and chat', () => {
    service.connect('ws://localhost:8080/ws');
    TestBed.flushEffects();

    expect(transportMock.send).toHaveBeenCalledTimes(1);
    const sentEvent = transportMock.send.mock.calls[0][0];
    expect(sentEvent.payload.case).toBe('loadHistory');
    if (sentEvent.payload.case === 'loadHistory') {
      expect(sentEvent.payload.value.spaceId).toBe('space-1');
      expect(sentEvent.payload.value.chatId).toBe('chat-1');
      expect(sentEvent.payload.value.cursor).toBe('');
      expect(sentEvent.payload.value.limit).toBe(50);
    }
  });

  it('should re-trigger loadHistory when activeChatId changes', () => {
    service.connect('ws://localhost:8080/ws');
    TestBed.flushEffects();
    transportMock.send.mockClear();

    workspaceMock.activeChatId.set('chat-2');
    TestBed.flushEffects();

    expect(transportMock.send).toHaveBeenCalledTimes(1);
    const sentEvent = transportMock.send.mock.calls[0][0];
    expect(sentEvent.payload.case).toBe('loadHistory');
    if (sentEvent.payload.case === 'loadHistory') {
      expect(sentEvent.payload.value.chatId).toBe('chat-2');
    }
  });

  it('should route sync_history into GraphStore projection logic and reset digests when empty', () => {
    const spyUpsert = vi.spyOn(graphStore, 'upsert');
    const spyReset = vi.spyOn(graphStore, 'resetProjection');
    cognitiveMock.activeDigests.set({ 'stale-1': { id: 'stale-1', summary: 'Old', isSticky: true } });
    
    service.connect('ws://localhost:8080/ws');

    const event = fromJson(WSEventSchema, {
      syncHistory: {
        spaceId: 'space-1',
        chatId: 'chat-1',
        projectionId: 'feature-A',
        recentEvents: [{ id: '1', timestamp: '2026-10-23T12:00:00Z', message: { type: 'PROMPT', text: 'test' } }],
        nextCursor: 'cursor-123',
        hasMore: true,
        digests: {},
        focus: { primaryFocus: [], relatedContext: [], dormantLenses: [] }
      }
    } as JsonValue);

    wsSubject.next(event);

    expect(spyUpsert).toHaveBeenCalled();
    expect(spyReset).toHaveBeenCalledWith({ spaceId: 'space-1', chatId: 'chat-1', projectionId: 'feature-A' }, ['1']);
    expect(timelineMock.setHistoryMetadata).toHaveBeenCalledWith('cursor-123', true, 'feature-A');
    expect(cognitiveMock.activeDigests()).toEqual({});
  });

  it('should preserve activeFocus during worker preview_context and not clear flow logs on approval', () => {
    const existingFocus: AttentionFocus = {
      primaryFocus: ['auth', 'middleware'],
      relatedContext: [],
      dormantLenses: []
    };
    cognitiveMock.activeFocus.set(existingFocus);

    service.connect('ws://localhost:8080/ws');

    const workerPreview = fromJson(WSEventSchema, {
      previewContext: {
        spaceId: 'space-1',
        chatId: 'chat-1',
        previewId: 'prev-worker',
        stage: 'CONTEXT_STAGE_WORKER',
        managerSystemPrompt: 'Worker rules',
        longTermMemory: ['Context digest'],
        recentHistory: ['[catalyst] Build auth'],
        fileContexts: [{ path: 'src/auth.go', level: 'injected', tokenCount: 120 }],
        focus: { primaryFocus: [], relatedContext: [], dormantLenses: [] }
      }
    } as JsonValue);

    wsSubject.next(workerPreview);

    expect(cognitiveMock.activeFocus()).toEqual(existingFocus);
    expect(cognitiveMock.pendingContextReview()?.stage).toBe(ContextStage.WORKER);

    service.submitContextApproval(true, ['src/auth.go'], [0]);
    expect(flowMock.resetTurn).not.toHaveBeenCalled();
    expect(cognitiveMock.pendingContextReview()).toBeNull();
  });

  it('should format switchBranch correctly and clear the GraphStore', () => {
    const spyGraphClear = vi.spyOn(graphStore, 'clear');
    
    service.switchBranch('feature-v2');
    
    expect(spyGraphClear).toHaveBeenCalled();
    expect(timelineMock.isLoadingHistory()).toBe(true);
    expect(transportMock.send).toHaveBeenCalled();
  });

  it('should handle request_transition mapping to targetThinkState', () => {
    service.connect('ws://localhost:8080/ws');

    const event = fromJson(WSEventSchema, {
      requestTransition: {
        targetThinkState: 'golang',
        reasoning: 'test'
      }
    } as JsonValue);

    wsSubject.next(event);
    
    expect(cognitiveMock.pendingTransition()).toEqual({ targetThinkState: 'golang', reasoning: 'test' });
  });

  it('should route sync_subgraph directly to the GraphStore using the coordinate', () => {
    const spyReset = vi.spyOn(graphStore, 'resetProjection');
    
    service.connect('ws://localhost:8080/ws');

    const event = fromJson(WSEventSchema, {
      syncSubGraph: {
        spaceId: 'space-1',
        chatId: 'chat-1',
        projectionId: 'flow-xyz',
        events: [{ id: 'sub-node', timestamp: '2026-10-23T12:00:00Z', message: { type: 'MODEL', text: 'data' } }]
      }
    } as JsonValue);

    wsSubject.next(event);

    expect(spyReset).toHaveBeenCalledWith({ spaceId: 'space-1', chatId: 'chat-1', projectionId: 'flow-xyz' }, ['sub-node']);
  });

  it('should append streamed_graph_event to the current projection list', () => {
    const spyAppend = vi.spyOn(graphStore, 'appendToProjection');
    
    service.connect('ws://localhost:8080/ws');

    const event = fromJson(WSEventSchema, {
      streamedGraphEvent: {
        spaceId: 'space-1',
        chatId: 'chat-1',
        projectionId: 'main',
        event: { id: 'stream-1', timestamp: '2026-10-23T12:00:00Z', message: { type: 'PROMPT', text: 'live' } }
      }
    } as JsonValue);

    wsSubject.next(event);

    expect(spyAppend).toHaveBeenCalledWith({ spaceId: 'space-1', chatId: 'chat-1', projectionId: 'main' }, ['stream-1']);
  });
});