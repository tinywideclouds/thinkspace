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

  it('should update connection status and initialize on connect', () => {
    service.connect('ws://test');
    expect(mockTransportService.connect).toHaveBeenCalledWith('ws://test');
    expect(service.connected()).toBe(true);
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

  it('should trigger pending strategy state on request_strategy', () => {
    service.connect('ws://test');
    
    const requestStrategyEvent = create(WSEventSchema, { payload: { case: 'requestStrategy', value: { active: true } } });
    webSocketSubject.next(requestStrategyEvent);

    expect(service.pendingStrategy()).toBe(true);
  });

  it('should trigger pending review state on request_review', () => {
    service.connect('ws://test');
    
    const requestReviewEvent = create(WSEventSchema, { payload: { case: 'requestReview', value: { branch: 'candidate/123' } } });
    webSocketSubject.next(requestReviewEvent);

    expect(service.pendingReviewBranch()).toBe('candidate/123');
  });

  it('should send prompt with correct space and chat ids', () => {
    service.submitPrompt('Refactor this app');
    
    expect(mockTransportService.send).toHaveBeenCalled();
    const chatHistory = service.coreChat();
    expect(chatHistory.length).toBe(2);
    expect(chatHistory[0].source).toBe('user');
    expect(chatHistory[0].content).toBe('Refactor this app');
    
    const sentProtocolBuffer = mockTransportService.send.mock.calls[0][0];
    expect(sentProtocolBuffer.payload.value.text).toBe('Refactor this app');
    expect(sentProtocolBuffer.payload.value.spaceId).toBe('golang');
    expect(sentProtocolBuffer.payload.value.chatId).toBe('test-chat');
  });

  it('should clear strategy state and send on selectStrategy', () => {
    service.pendingStrategy.set(true);
    service.selectStrategy(DomainDelegationStrategy.REFINE);
    
    expect(service.pendingStrategy()).toBe(false);
    expect(mockTransportService.send).toHaveBeenCalled();
  });

  it('should clear review state and send on submitReview', () => {
    service.pendingReviewBranch.set('candidate/123');
    service.submitReview(true);
    
    expect(service.pendingReviewBranch()).toBeNull();
    expect(mockTransportService.send).toHaveBeenCalled();
  });

  it('should clear the session completely on clearSession', () => {
    service.coreChat.set([{ id: '1', source: 'user', content: 'test' }]);
    service.pendingStrategy.set(true);
    service.inspectedReceipt.set('<xml/>');

    service.clearSession();

    expect(service.coreChat().length).toBe(0);
    expect(service.pendingStrategy()).toBe(false);
    expect(service.inspectedReceipt()).toBeNull();
  });

  it('should fetch flow receipt XML over HTTP with query parameters', async () => {
    const fetchPromise = service.inspectFlow('flow-999');
    
    const request = httpTestingController.expectOne('/api/receipts/test-chat/flow-999?space=golang');
    expect(request.request.method).toBe('GET');
    
    request.flush('<FlowReceipt FlowID="flow-999"></FlowReceipt>');
    
    await fetchPromise;
    expect(service.inspectedReceipt()).toBe('<FlowReceipt FlowID="flow-999"></FlowReceipt>');
  });

  it('should track agent stream by safely auto-initializing the activeAgents map', () => {
    service.connect('ws://test');
    
    // We removed agentStart, so we test that agentStream handles missing agent records cleanly
    webSocketSubject.next(create(WSEventSchema, { 
      payload: { case: 'agentStream', value: { agentId: 1, text: 'func main() {}' } } 
    }));

    const finalAgent = service.activeAgents().get(1);
    expect(finalAgent?.stream).toBe('func main() {}');
    expect(finalAgent?.status).toBe('running');
  });
});