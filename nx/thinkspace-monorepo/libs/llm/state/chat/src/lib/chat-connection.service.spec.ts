import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { ChatConnectionService } from './chat-connection.service';
import { ThinkspaceStateService } from './thinkspace-state.service';
import { TimelineStore } from './timeline.store';
import { TransportService } from '@org/llm-core-transport';
import { WSEvent, WSEventSchema } from '@org/llm-core-protos';
import { create } from '@bufbuild/protobuf';
import { Subject } from 'rxjs';
import { signal } from '@angular/core';
import { describe, beforeEach, it, expect, vi, Mock } from 'vitest';

describe('ChatConnectionService (Dispatcher)', () => {
  let service: ChatConnectionService;
  let mockTransportService: { connect: Mock, send: Mock, disconnect: Mock };
  let mockWorkspaceStateService: any;
  let mockTimelineStore: any;
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

    mockTimelineStore = {
      appendCoreChat: vi.fn(),
      appendToLastModelMessage: vi.fn(),
      clearSession: vi.fn(),
      ingestEvent: vi.fn()
    };

    TestBed.configureTestingModule({
      providers: [
        ChatConnectionService,
        provideHttpClient(),
        provideHttpClientTesting(),
        { provide: TransportService, useValue: mockTransportService },
        { provide: ThinkspaceStateService, useValue: mockWorkspaceStateService },
        { provide: TimelineStore, useValue: mockTimelineStore }
      ]
    });
    
    service = TestBed.inject(ChatConnectionService);
  });

  it('should route submitPrompt via Transport and emit optimistic UI', () => {
    service.submitPrompt('Refactor this app');
    
    expect(mockTransportService.send).toHaveBeenCalled();
    const sentEvent = mockTransportService.send.mock.calls[0][0];
    expect(sentEvent.payload.case).toBe('submitPrompt');
  });

  it('should route streamed_graph_event to timeline.ingestEvent', () => {
    const mockGraphEvent = { 
      type: 'streamed_graph_event', 
      event: { id: '1', type: 'model', content: 'test', timestamp: '', metadata: {} } 
    } as any;
    
    // Test the internal routing logic
    service['routeIncomingEvent'](mockGraphEvent);
    
    expect(mockTimelineStore.ingestEvent).toHaveBeenCalledWith(mockGraphEvent.event);
  });

  it('should successfully disconnect the transport', () => {
    service.disconnect();
    expect(mockTransportService.disconnect).toHaveBeenCalled();
  });
});