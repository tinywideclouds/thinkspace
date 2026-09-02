

### `libs\llm\state\chat\src\lib\chat-state.service.ts`
```
import { Injectable, signal, inject } from '@angular/core';
import { TransportService } from '@org/llm-core-transport';
import { LlmFacade, DomainEvent, DomainDelegationStrategy } from '@org/llm-core-facade';

export interface ChatItem {
  id: string;
  source: 'user' | 'model' | 'system';
  content: string;
}

export interface LogItem {
  id: string;
  timestamp: number;
  level: string;
  message: string;
}

export interface AgentState {
  id: number;
  instructions: string;
  stream: string;
  status: 'running' | 'completed';
  branch?: string;
  verified?: boolean;
}

@Injectable({ providedIn: 'root' })
export class ChatStateService {
  private transport = inject(TransportService);

  // Partitioned State Signals
  public coreChat = signal<ChatItem[]>([]);
  public systemLogs = signal<LogItem[]>([]);
  public activeAgents = signal<Map<number, AgentState>>(new Map());
  
  public spaces = signal<{id: string; name: string}[]>([]);
  public activeSpaceId = signal<string>('golang');
  public pendingStrategy = signal<boolean>(false);
  public pendingReviewBranch = signal<string | null>(null);
  public connected = signal<boolean>(false);

  public connect(url: string): void {
    this.transport.connect(url).subscribe({
      next: (proto) => {
        const domainEvent = LlmFacade.toDomain(proto);
        if (domainEvent) {
          this.handleEvent(domainEvent);
        }
      },
      error: (err) => {
        this.appendSystemLog('ERROR', `Connection error: ${err}`);
        this.connected.set(false);
      },
      complete: () => {
        this.appendSystemLog('INFO', 'Connection closed.');
        this.connected.set(false);
      }
    });
    this.connected.set(true);
  }

  public disconnect(): void {
    this.transport.disconnect();
    this.connected.set(false);
  }

  public submitPrompt(text: string): void {
    this.appendCoreChat('user', text);
    const proto = LlmFacade.createSubmitPrompt(text, this.activeSpaceId());
    this.transport.send(proto);
  }

  private handleEvent(event: DomainEvent): void {
    switch (event.type) {
      case 'available_spaces':
        this.spaces.set(event.spaces);
        this.appendSystemLog('INFO', `Available spaces: ${event.spaces.map(s => s.id).join(', ')}`);
        break;
      
      case 'chat_stream':
        this.appendToLastModelMessage(event.text);
        break;
      
      case 'log_message':
        this.appendSystemLog(event.level, event.message);
        break;
      
      case 'delegation_start':
        // Log to system, but ALSO drop a contextual note in the main chat feed
        const msg = `🚀 Delegating task to ${event.agentCount} agent(s): ${event.instructions}`;
        this.appendSystemLog('INFO', msg);
        this.appendCoreChat('system', msg);
        break;
      
      case 'delegation_complete':
        this.appendSystemLog('INFO', `✅ Delegation Flow Complete:\n${event.summary}`);
        break;
      
      case 'agent_start':
        this.activeAgents.update(map => {
          const newMap = new Map(map);
          newMap.set(event.agentId, {
            id: event.agentId,
            instructions: event.instructions,
            stream: '',
            status: 'running'
          });
          return newMap;
        });
        break;
      
      case 'agent_stream':
        this.activeAgents.update(map => {
          const newMap = new Map(map);
          const agent = newMap.get(event.agentId);
          if (agent) {
            agent.stream += event.text;
            newMap.set(event.agentId, agent);
          }
          return newMap;
        });
        break;
      
      case 'agent_complete':
        this.activeAgents.update(map => {
          const newMap = new Map(map);
          const agent = newMap.get(event.agentId);
          if (agent) {
            agent.status = 'completed';
            agent.branch = event.branch;
            agent.verified = event.verified;
            newMap.set(event.agentId, agent);
          }
          return newMap;
        });
        break;
      
      case 'request_strategy':
        this.pendingStrategy.set(event.active);
        break;
      
      case 'request_review':
        this.pendingReviewBranch.set(event.branch);
        break;
    }
  }

  private appendCoreChat(source: ChatItem['source'], content: string): void {
    this.coreChat.update(chat => [...chat, { id: crypto.randomUUID(), source, content }]);
  }

  private appendToLastModelMessage(chunk: string): void {
    this.coreChat.update(chat => {
      if (chat.length === 0 || chat[chat.length - 1].source !== 'model') {
        return [...chat, { id: crypto.randomUUID(), source: 'model', content: chunk }];
      }
      const newChat = [...chat];
      newChat[newChat.length - 1].content += chunk;
      return newChat;
    });
  }

  private appendSystemLog(level: string, message: string): void {
    this.systemLogs.update(logs => [...logs, { 
      id: crypto.randomUUID(), 
      timestamp: Date.now(),
      level, 
      message 
    }]);
  }

  public selectStrategy(strategy: DomainDelegationStrategy): void {
    this.pendingStrategy.set(false);
    // Explicitly record the user's decision in the main chat feed
    this.appendCoreChat('user', `Selected Strategy: ${DomainDelegationStrategy[strategy]}`);
    
    const proto = LlmFacade.createSelectStrategy(strategy);
    this.transport.send(proto);
  }

  public submitReview(accepted: boolean): void {
    const branch = this.pendingReviewBranch();
    if (!branch) return;
    
    this.pendingReviewBranch.set(null);
    // Explicitly record the user's branch decision in the main chat feed
    this.appendCoreChat('user', `Review for ${branch}: ${accepted ? 'ACCEPTED' : 'REJECTED'}`);
    
    const proto = LlmFacade.createReviewDecision(branch, accepted);
    this.transport.send(proto);
  }
}
```


### `libs\llm\state\chat\src\lib\chat-state.service.spec.ts`
```
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
```


### `libs\llm\core\facade\src\lib\domain-models.ts`
```
export type DomainEvent =
  | { type: 'chat_stream'; text: string }
  | { type: 'log_message'; level: string; message: string }
  | { type: 'delegation_start'; agentCount: number; instructions: string }
  | { type: 'delegation_complete'; summary: string }
  | { type: 'agent_start'; agentId: number; instructions: string }
  | { type: 'agent_stream'; agentId: number; text: string }
  | { type: 'agent_complete'; agentId: number; branch: string; verified: boolean }
  | { type: 'request_strategy'; active: boolean }
  | { type: 'request_review'; branch: string }
  | { type: 'available_spaces'; spaces: { id: string; name: string }[] };

export enum DomainDelegationStrategy {
  SKIP = 1,
  MANUAL = 2,
  REVIEW = 3,
  REFINE = 4,
}
```


### `libs\llm\core\facade\src\lib\facade.spec.ts`
```
import { describe, it, expect } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { WSEventSchema, DelegationStrategy } from '@org/llm-core-protos';
import { LlmFacade } from './facade';
import { DomainDelegationStrategy } from './domain-models';

describe('LlmFacade', () => {
  describe('toDomain (Inbound)', () => {
    it('should map a chat stream event correctly', () => {
      const proto = create(WSEventSchema, {
  payload: {
    case: 'chatStream',
    value: { text: 'Hello from model' }
  }
});
      const domain = LlmFacade.toDomain(proto);
      expect(domain).toEqual({ type: 'chat_stream', text: 'Hello from model' });
    });

    it('should map an available spaces event correctly', () => {
      const proto = create(WSEventSchema, {
        payload: {
          case: 'availableSpaces',
          value: {
            spaces: [
              { id: 'golang', name: 'Go Developer' },
              { id: 'angular', name: 'Angular Expert' }
            ]
          }
        }
      });

      const domain = LlmFacade.toDomain(proto);
      expect(domain).toEqual({
        type: 'available_spaces',
        spaces: [
          { id: 'golang', name: 'Go Developer' },
          { id: 'angular', name: 'Angular Expert' }
        ]
      });
    });

    it('should return null for undefined payload or case', () => {
      const emptyProto = create(WSEventSchema);
      expect(LlmFacade.toDomain(emptyProto)).toBeNull();
    });
  });

  describe('Outbound Builders', () => {
    it('should create a valid SubmitPrompt WSEvent', () => {
      const proto = LlmFacade.createSubmitPrompt('Write a test', 'golang');
      
      expect(proto.payload.case).toBe('submitPrompt');
      if (proto.payload.case === 'submitPrompt') {
        expect(proto.payload.value.text).toBe('Write a test');
        expect(proto.payload.value.spaceId).toBe('golang');
      }
    });

    it('should create a valid SelectStrategy WSEvent', () => {
      const proto = LlmFacade.createSelectStrategy(DomainDelegationStrategy.REFINE);
      
      expect(proto.payload.case).toBe('selectStrategy');
      if (proto.payload.case === 'selectStrategy') {
        // Asserting it maps correctly to the underlying proto enum value (4)
        expect(proto.payload.value.strategyId).toBe(DelegationStrategy.REFINE);
      }
    });

    it('should create a valid ReviewDecision WSEvent', () => {
      const proto = LlmFacade.createReviewDecision('candidate/123', true);
      
      expect(proto.payload.case).toBe('reviewDecision');
      if (proto.payload.case === 'reviewDecision') {
        expect(proto.payload.value.branch).toBe('candidate/123');
        expect(proto.payload.value.accepted).toBe(true);
      }
    });
  });
});
```


### `libs\llm\core\facade\src\lib\facade.ts`
```
import { create } from '@bufbuild/protobuf';
import { 
  WSEvent, 
  WSEventSchema,
  DelegationStrategy 
} from '@org/llm-core-protos';
import { DomainEvent, DomainDelegationStrategy } from './domain-models';

export class LlmFacade {
  /**
   * Translates an inbound Protobuf WSEvent into a clean DomainEvent.
   * Leverages the `oneof` case for strict type inference.
   */
  static toDomain(proto: WSEvent): DomainEvent | null {
    if (!proto.payload || proto.payload.case === undefined) {
      return null;
    }

    switch (proto.payload.case) {
      case 'chatStream':
        return { type: 'chat_stream', text: proto.payload.value.text };
      
      case 'logMessage':
        return { type: 'log_message', level: proto.payload.value.level, message: proto.payload.value.message };
      
      case 'delegationStart':
        return { 
          type: 'delegation_start', 
          agentCount: proto.payload.value.agentCount, 
          instructions: proto.payload.value.instructions 
        };
      
      case 'delegationComplete':
        return { type: 'delegation_complete', summary: proto.payload.value.summary };
      
      case 'agentStart':
        return { 
          type: 'agent_start', 
          agentId: proto.payload.value.agentId, 
          instructions: proto.payload.value.instructions 
        };
      
      case 'agentStream':
        return { type: 'agent_stream', agentId: proto.payload.value.agentId, text: proto.payload.value.text };
      
      case 'agentComplete':
        return { 
          type: 'agent_complete', 
          agentId: proto.payload.value.agentId, 
          branch: proto.payload.value.branch, 
          verified: proto.payload.value.verified 
        };
      
      case 'requestStrategy':
        return { type: 'request_strategy', active: proto.payload.value.active };
      
      case 'requestReview':
        return { type: 'request_review', branch: proto.payload.value.branch };
      
      case 'availableSpaces':
        return { 
          type: 'available_spaces', 
          spaces: proto.payload.value.spaces.map(s => ({ id: s.id, name: s.name })) 
        };
      
      default:
        console.warn(`[LlmFacade] Unhandled inbound proto case: ${proto.payload.case}`);
        return null;
    }
  }

  /**
   * Wraps a user prompt into the outbound Protobuf envelope safely using the schema.
   */
  static createSubmitPrompt(text: string, spaceId: string): WSEvent {
    return create(WSEventSchema, {
      payload: {
        case: 'submitPrompt',
        value: { text, spaceId }
      }
    });
  }

  /**
   * Wraps a user's strategy selection into the outbound Protobuf envelope safely using the schema.
   */
  static createSelectStrategy(strategy: DomainDelegationStrategy): WSEvent {
    const strategyId = strategy as unknown as DelegationStrategy;
    
    return create(WSEventSchema, {
      payload: {
        case: 'selectStrategy',
        value: { strategyId }
      }
    });
  }

  /**
   * Wraps a user's review decision into the outbound Protobuf envelope safely using the schema.
   */
  static createReviewDecision(branch: string, accepted: boolean): WSEvent {
    return create(WSEventSchema, {
      payload: {
        case: 'reviewDecision',
        value: { branch, accepted }
      }
    });
  }
}
```
