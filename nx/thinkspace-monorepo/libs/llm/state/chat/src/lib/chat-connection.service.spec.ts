import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { ChatConnectionService } from './chat-connection.service';
import { WorkspaceStateService } from './workspace-state.service';
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
        ChatConnectionService,
        provideHttpClient(),
        provideHttpClientTesting(),
        { provide: TransportService, useValue: mockTransportService },
        { provide: WorkspaceStateService, useValue: mockWorkspaceStateService }
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

  it('should successfully disconnect the transport', () => {
    service.disconnect();
    expect(mockTransportService.disconnect).toHaveBeenCalled();
  });
});