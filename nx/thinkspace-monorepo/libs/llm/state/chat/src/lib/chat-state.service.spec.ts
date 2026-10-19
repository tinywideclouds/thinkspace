import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { ChatStateService } from './chat-state.service';
import { WorkspaceStateService } from './workspace-state.service';
import { TransportService } from '@org/llm-core-transport';
import { DomainDelegationStrategy } from '@org/llm-core-facade';
import { WSEvent, WSEventSchema } from '@org/llm-core-protos';
import { create } from '@bufbuild/protobuf';
import { Subject } from 'rxjs';
import { signal } from '@angular/core';
import { describe, beforeEach, it, expect, vi, Mock } from 'vitest';

describe('ChatStateService', () => {
  let service: ChatStateService;
  let httpTestingController: HttpTestingController;
  let mockTransportService: { connect: Mock, send: Mock, disconnect: Mock };
  let mockWorkspaceStateService: any;
  let webSocketSubject: Subject<WSEvent>;

  beforeEach(() => {
    webSocketSubject = new Subject<WSEvent>();
    
    mockTransportService = {
      connect: vi.fn(() => webSocketSubject.asObservable()),
      send: vi.fn(),
      disconnect: vi.fn(),
    };

    mockWorkspaceStateService = {
      activeSpaceId: signal('golang'),
      activeChatId: signal('test-chat')
    };

    TestBed.configureTestingModule({
      providers: [
        ChatStateService,
        provideHttpClient(),
        provideHttpClientTesting(),
        { provide: TransportService, useValue: mockTransportService },
        { provide: WorkspaceStateService, useValue: mockWorkspaceStateService }
      ]
    });
    
    service = TestBed.inject(ChatStateService);
    httpTestingController = TestBed.inject(HttpTestingController);
  });

  it('should process syncHistory to populate coreChat, focus, and digests', () => {
    service.connect('ws://test');
    
    const syncEvent = create(WSEventSchema, {
      payload: {
        case: 'syncHistory',
        value: {
          recentEvents: [
            { id: '1', type: 'prompt', content: 'historical prompt', timestamp: '2026-09-02T15:00:00Z', metadata: {} },
            { id: '2', type: 'model', content: 'historical model', timestamp: '2026-09-02T15:01:00Z', metadata: {} }
          ],
          digests: {
            'uuid-1': { id: 'uuid-1', summary: 'test digest', isSticky: true }
          },
          focus: {
            primaryFocus: ['golang'],
            relatedContext: ['auth'],
            dormantLenses: []
          }
        }
      }
    });

    webSocketSubject.next(syncEvent);

    const chatHistory = service.coreChat();
    expect(chatHistory.length).toBe(2);
    expect(chatHistory[0].source).toBe('user');
    expect(chatHistory[0].content).toBe('historical prompt');
    
    expect(service.activeFocus()?.primaryFocus).toEqual(['golang']);
    expect(service.activeDigests()['uuid-1'].summary).toBe('test digest');
  });

  it('should process previewContext to populate focus and workbench', () => {
    service.connect('ws://test');
    
    const previewEvent = create(WSEventSchema, {
      payload: {
        case: 'previewContext',
        value: {
          spaceId: 'golang',
          chatId: 'test-chat',
          managerSystemPrompt: 'test',
          stickyDigests: [],
          recentHistory: [],
          fileContexts: [
            { path: 'src/main.go', level: 'structural', tokenCount: 100 }
          ],
          focus: {
            primaryFocus: ['database'],
            relatedContext: [],
            dormantLenses: []
          }
        }
      }
    });

    // Disable auto-approve to test state assignment
    service.autoApproveContext.set(false);
    webSocketSubject.next(previewEvent);

    expect(service.activeFocus()?.primaryFocus).toEqual(['database']);
    expect(service.activeWorkbench().length).toBe(1);
    expect(service.activeWorkbench()[0].path).toBe('src/main.go');
  });

  it('should stream model chunks into a single coreChat item', () => {
    service.connect('ws://test');
    
    const firstChunk = create(WSEventSchema, { payload: { case: 'chatStream', value: { text: 'Hello ' } } });
    const secondChunk = create(WSEventSchema, { payload: { case: 'chatStream', value: { text: 'World' } } });
    
    webSocketSubject.next(firstChunk);
    webSocketSubject.next(secondChunk);

    const chatHistory = service.coreChat();
    expect(chatHistory.length).toBe(1);
    expect(chatHistory[0].source).toBe('model');
    expect(chatHistory[0].content).toBe('Hello World');
  });

  it('should send prompt, clear logs, and show thinking indicator', () => {
    service.systemLogs.set([{ id: 'old', timestamp: 0, level: 'INFO', message: 'old log' }]);
    
    service.submitPrompt('Refactor this app');
    
    expect(mockTransportService.send).toHaveBeenCalled();
    
    const chatHistory = service.coreChat();
    expect(chatHistory.length).toBe(2);
    expect(chatHistory[0].source).toBe('user');
    expect(chatHistory[0].content).toBe('Refactor this app');
    expect(chatHistory[1].source).toBe('system');
    expect(chatHistory[1].content).toBe('Manager Thinking...');
    
    expect(service.systemLogs().length).toBe(0);

    const sentProtocolBuffer = mockTransportService.send.mock.calls[0][0];
    expect(sentProtocolBuffer.payload.value.text).toBe('Refactor this app');
  });

  it('should handle request_transition and submitTransitionDecision', () => {
    service.connect('ws://test');

    const reqEvent = create(WSEventSchema, {
      payload: {
        case: 'requestTransition',
        value: { targetDomain: 'golang', reasoning: 'need compilation' }
      }
    });
    webSocketSubject.next(reqEvent);

    expect(service.pendingTransition()).toEqual({ targetDomain: 'golang', reasoning: 'need compilation' });

    service.submitTransitionDecision(true);

    expect(service.pendingTransition()).toBeNull();
    expect(mockTransportService.send).toHaveBeenCalled();

    const sentEvent = mockTransportService.send.mock.calls[0][0];
    expect(sentEvent.payload.case).toBe('transitionDecision');
    if (sentEvent.payload.case === 'transitionDecision') {
      expect(sentEvent.payload.value.approved).toBe(true);
    }
  });

  it('should inject flow card immediately on flow_start and replace thinking indicator', () => {
    service.connect('ws://test');
    
    service.coreChat.set([{ id: '1', source: 'system', content: 'Manager Thinking...' }]);

    const startEvent = create(WSEventSchema, { 
      payload: { 
        case: 'telemetryEvent', 
        value: { 
          channel: 'manager',
          event: { 
            id: 'evt-1',
            type: 'flow_start', 
            summary: 'Refactoring Auth',
            payloadJson: '{"flow_id": "flow-123", "task_id": "test", "agent_count": 1}' 
          }
        } 
      } 
    });
    webSocketSubject.next(startEvent);

    expect(service.coreChat().length).toBe(1);
    expect(service.coreChat()[0].source).toBe('flow_card');
    expect(service.coreChat()[0].content).toBe('Refactoring Auth');
    expect(service.coreChat()[0].status).toBe('running');
  });

  it('should maintain an append-only timeline for agents via immutable updates', () => {
    service.connect('ws://test');
    
    webSocketSubject.next(create(WSEventSchema, { 
      payload: { case: 'telemetryEvent', value: { channel: 'manager', event: { type: 'flow_start', payloadJson: '{"flow_id": "f1", "agent_count": 1}' } } } 
    }));
    
    webSocketSubject.next(create(WSEventSchema, { 
      payload: { case: 'telemetryEvent', value: { channel: 'agent-1', event: { type: 'flow_status', message: 'executing', payloadJson: '{"flow_id": "f1", "agent_index": 1}' } } } 
    }));
    
    webSocketSubject.next(create(WSEventSchema, { 
      payload: { case: 'telemetryEvent', value: { channel: 'agent-1', event: { type: 'flow_status', message: 'running_tests', payloadJson: '{"flow_id": "f1", "agent_index": 1}' } } } 
    }));
    
    webSocketSubject.next(create(WSEventSchema, { 
      payload: { case: 'telemetryEvent', value: { channel: 'agent-1', event: { type: 'flow_error', message: 'timeout error', payloadJson: '{"flow_id": "f1", "agent_index": 1, "trace": "timeout error"}' } } } 
    }));

    const agent = service.flowStates().get('f1')?.agents.get('agent-1');
    expect(agent?.timeline.length).toBe(3);
    expect(agent?.timeline[0].message).toContain('executing');
    expect(agent?.timeline[2].isError).toBe(true);
    expect(agent?.timeline[2].message).toContain('timeout error');
    expect(agent?.status).toBe('failed');
  });

  it('should mark flow complete and update card status when all agents finish', () => {
    service.connect('ws://test');
    
    webSocketSubject.next(create(WSEventSchema, { 
      payload: { case: 'telemetryEvent', value: { channel: 'manager', event: { type: 'flow_start', payloadJson: '{"flow_id": "f1", "agent_count": 2}' } } } 
    }));

    webSocketSubject.next(create(WSEventSchema, { 
      payload: { case: 'telemetryEvent', value: { channel: 'manager', event: { type: 'flow_complete', payloadJson: '{"flow_id": "f1", "agent_index": 1, "passed": true}' } } } 
    }));

    expect(service.flowStates().get('f1')?.status).toBe('running');
    expect(service.coreChat()[0].status).toBe('running');

    webSocketSubject.next(create(WSEventSchema, { 
      payload: { case: 'telemetryEvent', value: { channel: 'manager', event: { type: 'flow_complete', payloadJson: '{"flow_id": "f1", "agent_index": 2, "passed": false}' } } } 
    }));

    expect(service.flowStates().get('f1')?.status).toBe('completed');
    expect(service.coreChat()[0].status).toBe('completed');
  });

  it('should log manager telemetry events into system logs', () => {
    service.connect('ws://test');
    
    webSocketSubject.next(create(WSEventSchema, { 
      payload: { case: 'telemetryEvent', value: { channel: 'manager', event: { type: 'log', message: 'Analyzing context...' } } } 
    }));

    expect(service.systemLogs().length).toBe(1);
    expect(service.systemLogs()[0].message).toBe('Analyzing context...');
    expect(service.systemLogs()[0].level).toBe('INFO');
  });
});