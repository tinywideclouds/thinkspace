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