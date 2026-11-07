import { TestBed } from '@angular/core/testing';
import { ChatConnectionService } from './chat-connection.service';
import { TransportService } from '@org/llm-core-transport';
import { GraphStore } from './graph.store';
import { TimelineStore } from './timeline.store';
import { CognitiveStore } from './cognitive.store';
import { FlowStore } from './flow.store';
import { ThinkspaceStateService } from './thinkspace-state.service';
import { UiSettingsService } from './ui-settings.service';
import { Subject } from 'rxjs';
import { PreviewContext, AttentionFocus, DigestMeta, FileContextMetaData, GraphEvent, BranchTopology } from '@org/llm-core-facade';
import { WSEvent, WSEventSchema } from '@org/llm-core-protos';
import { create } from '@bufbuild/protobuf';
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
  activeBranch: WritableSignal<string>;
  branches: WritableSignal<Record<string, BranchTopology>>;
  nextCursor: WritableSignal<string>;
  hasMore: WritableSignal<boolean>;
  coreChat: Signal<ChatItem[]>;
  setHistoryMetadata: Mock<(cursor: string, more: boolean, branch: string, topology: Record<string, BranchTopology>) => void>;
}

interface MockThinkspaceStateService {
  activeSpaceId: WritableSignal<string | null>;
  activeChatId: WritableSignal<string | null>;
}

interface MockFlowStore {
  resetTurn: Mock<() => void>;
  clearSession: Mock<() => void>;
  appendSystemLog: Mock<(level: string, msg: string) => void>;
  processTelemetry: Mock<(event: any) => void>;
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

  const mockTime = { toString: () => '2026-10-23T12:00:00Z' } as any;

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
      activeBranch: signal('main'),
      branches: signal({}),
      nextCursor: signal(''),
      hasMore: signal(true),
      coreChat: computed(() => []),
      setHistoryMetadata: vi.fn()
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

  it('should initialize connection and set connected true upon receiving WSEvent', () => {
    expect(transportMock.connect).toHaveBeenCalled();
    expect(service.connected()).toBe(false);

    const event = create(WSEventSchema, {
      payload: {
        case: 'chatStream',
        value: { text: 'chunk' }
      }
    });

    wsSubject.next(event);
    expect(service.connected()).toBe(true);
  });

  it('should route sync_history directly to the GraphStore and update metadata', () => {
    const spyUpsert = vi.spyOn(graphStore, 'upsert');
    const mockNode: GraphEvent = { id: '1', type: 'prompt', content: 'test', timestamp: mockTime, tags: [], tombstoned: false, metadata: {}, edges: {} };

    service.handleEvent({
      type: 'sync_history',
      recentEvents: [mockNode],
      nextCursor: 'cursor-123',
      hasMore: true,
      activeBranch: 'feature-A',
      branches: {},
      digests: {},
      focus: { primaryFocus: [], relatedContext: [], dormantLenses: [] }
    });

    expect(spyUpsert).toHaveBeenCalled();
    expect(timelineMock.setHistoryMetadata).toHaveBeenCalledWith('cursor-123', true, 'feature-A', {});
  });

  it('should route sync_subgraph directly to the GraphStore', () => {
    const spyUpsert = vi.spyOn(graphStore, 'upsert');
    const mockNode: GraphEvent = { id: 'sub-node', type: 'model', content: 'data', timestamp: mockTime, tags: [], tombstoned: false, metadata: {}, edges: {} };

    service.handleEvent({
      type: 'sync_subgraph',
      events: [mockNode]
    });

    expect(spyUpsert).toHaveBeenCalled();
  });

  it('should route streamed_graph_event directly to the GraphStore', () => {
    const spyUpsert = vi.spyOn(graphStore, 'upsert');
    const mockNode: GraphEvent = { id: 'stream-1', type: 'system', content: 'hello', timestamp: mockTime, tags: [], tombstoned: false, metadata: {}, edges: {} };

    service.handleEvent({
      type: 'streamed_graph_event',
      event: mockNode
    });

    expect(spyUpsert).toHaveBeenCalled();
  });

  it('should format switchBranch correctly and send via transport', () => {
    service.switchBranch('feature-v2');

    expect(transportMock.send).toHaveBeenCalled();
    const payload = transportMock.send.mock.calls[0][0];
    expect(payload.payload.case).toBe('switchBranch');
  });

  it('should handle request_transition mapping to targetThinkState', () => {
    service.handleEvent({
      type: 'request_transition',
      targetThinkState: 'golang',
      reasoning: 'test'
    });

    expect(cognitiveMock.pendingTransition()).toEqual({ targetThinkState: 'golang', reasoning: 'test' });
  });

  it('should send a loadSubGraph command when requested', () => {
    service.loadSubGraph('flow-xyz');
    expect(transportMock.send).toHaveBeenCalled();

    const payload = transportMock.send.mock.calls[0][0];
    expect(payload.payload.case).toBe('loadSubGraph');
    if (payload.payload.case === 'loadSubGraph') {
      expect(payload.payload.value.graphId).toBe('flow-xyz');
    }
  });

  it('should send a LoadHistory command on pagination', () => {
    timelineMock.nextCursor.set('prev-cursor');

    service.loadHistoryPage();

    expect(timelineMock.isLoadingHistory()).toBe(true);
    expect(transportMock.send).toHaveBeenCalled();

    const payload = transportMock.send.mock.calls[0][0];
    expect(payload.payload.case).toBe('loadHistory');
    if (payload.payload.case === 'loadHistory') {
      expect(payload.payload.value.cursor).toBe('prev-cursor');
    }
  });

  it('should clear all session state across stores on clearSession()', () => {
    const spyGraphClear = vi.spyOn(graphStore, 'clear');

    service.clearSession();

    expect(spyGraphClear).toHaveBeenCalled();
    expect(flowMock.manualClearLogs).toHaveBeenCalled();
    expect(cognitiveMock.clearSession).toHaveBeenCalled();
    expect(timelineMock.setHistoryMetadata).toHaveBeenCalledWith('', false, 'main', {});
  });
});