import { TestBed } from '@angular/core/testing';
import { ChatStateService } from './chat-state.service';
import { TransportService } from '@org/llm-core-transport';
import { DomainDelegationStrategy } from '@org/llm-core-facade';
import { WSEvent, WSEventSchema } from '@org/llm-core-protos';
import { create } from '@bufbuild/protobuf';
import { Subject } from 'rxjs';
import { describe, beforeEach, it, expect, vi, Mock } from 'vitest';

describe('ChatStateService', () => {
  let service: ChatStateService;
  let mockTransport: { connect: Mock, send: Mock, disconnect: Mock };
  let wsSubject: Subject<WSEvent>;

  beforeEach(() => {
    wsSubject = new Subject<WSEvent>();
    
    mockTransport = {
      connect: vi.fn(() => wsSubject.asObservable()),
      send: vi.fn(),
      disconnect: vi.fn(),
    };

    TestBed.configureTestingModule({
      providers: [
        ChatStateService,
        { provide: TransportService, useValue: mockTransport }
      ]
    });
    service = TestBed.inject(ChatStateService);
  });

  it('should update connection status and initialize on connect', () => {
    service.connect('ws://test');
    expect(mockTransport.connect).toHaveBeenCalledWith('ws://test');
    expect(service.connected()).toBe(true);
  });

  it('should stream model chunks into a single coreChat item', () => {
    service.connect('ws://test');
    
    const chunk1 = create(WSEventSchema, { payload: { case: 'chatStream', value: { text: 'Hello ' } } });
    const chunk2 = create(WSEventSchema, { payload: { case: 'chatStream', value: { text: 'World' } } });
    
    wsSubject.next(chunk1);
    wsSubject.next(chunk2);

    const chat = service.coreChat();
    expect(chat.length).toBe(1);
    expect(chat[0].source).toBe('model');
    expect(chat[0].content).toBe('Hello World');
  });

  it('should trigger pending strategy state on request_strategy', () => {
    service.connect('ws://test');
    
    const reqStrategy = create(WSEventSchema, { payload: { case: 'requestStrategy', value: { active: true } } });
    wsSubject.next(reqStrategy);

    expect(service.pendingStrategy()).toBe(true);
  });

  it('should trigger pending review state on request_review', () => {
    service.connect('ws://test');
    
    const reqReview = create(WSEventSchema, { payload: { case: 'requestReview', value: { branch: 'candidate/123' } } });
    wsSubject.next(reqReview);

    expect(service.pendingReviewBranch()).toBe('candidate/123');
  });

  it('should send prompt and append user message to coreChat', () => {
    service.submitPrompt('Refactor this app');
    
    expect(mockTransport.send).toHaveBeenCalled();
    const chat = service.coreChat();
    expect(chat.length).toBe(1);
    expect(chat[0].source).toBe('user');
    expect(chat[0].content).toBe('Refactor this app');
  });

  it('should clear strategy state and send on selectStrategy', () => {
    service.pendingStrategy.set(true);
    service.selectStrategy(DomainDelegationStrategy.REFINE);
    
    expect(service.pendingStrategy()).toBe(false);
    expect(mockTransport.send).toHaveBeenCalled();
  });

  it('should clear review state and send on submitReview', () => {
    service.pendingReviewBranch.set('candidate/123');
    service.submitReview(true);
    
    expect(service.pendingReviewBranch()).toBeNull();
    expect(mockTransport.send).toHaveBeenCalled();
  });

  it('should route delegation start to both systemLogs and coreChat', () => {
    service.connect('ws://test');
    
    wsSubject.next(create(WSEventSchema, { 
      payload: { case: 'delegationStart', value: { agentCount: 2, instructions: 'Do work' } } 
    }));

    expect(service.systemLogs().length).toBe(1);
    expect(service.systemLogs()[0].message).toContain('Delegating task to 2');
    
    expect(service.coreChat().length).toBe(1);
    expect(service.coreChat()[0].source).toBe('system');
    expect(service.coreChat()[0].content).toContain('Delegating task to 2');
  });

  it('should track agent lifecycle in activeAgents map', () => {
    service.connect('ws://test');
    
    // Start Agent
    wsSubject.next(create(WSEventSchema, { 
      payload: { case: 'agentStart', value: { agentId: 1, instructions: 'Code something' } } 
    }));
    
    expect(service.activeAgents().has(1)).toBe(true);
    expect(service.activeAgents().get(1)?.status).toBe('running');

    // Stream text to Agent
    wsSubject.next(create(WSEventSchema, { 
      payload: { case: 'agentStream', value: { agentId: 1, text: 'func main() {}' } } 
    }));

    expect(service.activeAgents().get(1)?.stream).toBe('func main() {}');

    // Complete Agent
    wsSubject.next(create(WSEventSchema, { 
      payload: { case: 'agentComplete', value: { agentId: 1, branch: 'feat/test', verified: true } } 
    }));

    const finalAgent = service.activeAgents().get(1);
    expect(finalAgent?.status).toBe('completed');
    expect(finalAgent?.branch).toBe('feat/test');
    expect(finalAgent?.verified).toBe(true);
  });
});