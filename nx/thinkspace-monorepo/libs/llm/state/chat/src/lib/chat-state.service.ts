import { Injectable, signal, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { TransportService } from '@org/llm-core-transport';
import { LlmFacade, DomainEvent, DomainDelegationStrategy, DomainFlowEvent } from '@org/llm-core-facade';
import { firstValueFrom } from 'rxjs';
import { WorkspaceStateService } from './workspace-state.service';

export interface ChatItem {
  id: string;
  source: 'user' | 'model' | 'system' | 'flow_card';
  content: string;
  flowId?: string;
  status?: 'running' | 'completed'; 
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

export interface TimelineEntry {
  id: string;
  timestamp: number;
  message: string;
  isError: boolean;
}

export interface FlowAgentState {
  agentId: string;
  agentIndex: number;
  instruction: string;
  status: string;
  attempt: number;
  trace: string;
  passed: boolean;
  timeline: TimelineEntry[];
}

export interface FlowState {
  flowId: string;
  taskId: string;
  agentCount: number;
  completedCount: number;
  status: 'running' | 'completed';
  agents: Map<string, FlowAgentState>;
}

@Injectable({ providedIn: 'root' })
export class ChatStateService {
  private transportService = inject(TransportService);
  private httpClient = inject(HttpClient);
  private workspaceStateService = inject(WorkspaceStateService); 

  public coreChat = signal<ChatItem[]>([]);
  public systemLogs = signal<LogItem[]>([]);
  public activeAgents = signal<Map<number, AgentState>>(new Map());
  public flowStates = signal<Map<string, FlowState>>(new Map());
  
  public pendingStrategy = signal<boolean>(false);
  public pendingReviewBranch = signal<string | null>(null);
  public connected = signal<boolean>(false);
  public inspectedReceipt = signal<string | null>(null);

  public connect(url: string): void {
    this.transportService.connect(url).subscribe({
      next: (protocolBufferEvent) => {
        const domainEvent = LlmFacade.toDomain(protocolBufferEvent);
        if (domainEvent) {
          this.handleEvent(domainEvent);
        }
      },
      error: (error) => {
        this.appendSystemLog('ERROR', `Connection error: ${error}`);
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
    this.transportService.disconnect();
    this.connected.set(false);
  }

  public clearSession(): void {
    this.coreChat.set([]);
    this.systemLogs.set([]);
    this.activeAgents.set(new Map());
    this.flowStates.set(new Map());
    this.pendingStrategy.set(false);
    this.pendingReviewBranch.set(null);
    this.inspectedReceipt.set(null);
  }

  public submitPrompt(text: string): void {
    const spaceId = this.workspaceStateService.activeSpaceId();
    const chatId = this.workspaceStateService.activeChatId();

    if (!spaceId || !chatId) {
      this.appendSystemLog('ERROR', 'Cannot submit prompt: No active space or chat selected.');
      return;
    }

    this.appendCoreChat('user', text);
    
    const protocolBufferEvent = LlmFacade.createSubmitPrompt(text, spaceId, chatId);
    this.transportService.send(protocolBufferEvent);
  }

  public async inspectFlow(flowId: string): Promise<void> {
    const spaceId = this.workspaceStateService.activeSpaceId();
    const chatId = this.workspaceStateService.activeChatId();

    if (!spaceId || !chatId) return;

    try {
      const url = `/api/receipts/${encodeURIComponent(chatId)}/${encodeURIComponent(flowId)}?space=${encodeURIComponent(spaceId)}`;
      const xmlData = await firstValueFrom(this.httpClient.get(url, { responseType: 'text' }));
      this.inspectedReceipt.set(xmlData);
    } catch (error) {
      console.error(`Failed to fetch receipt for flow ${flowId}`, error);
      this.appendSystemLog('ERROR', `Failed to load receipt for flow ${flowId}`);
    }
  }

  private handleEvent(event: DomainEvent): void {
    switch (event.type) {
      case 'available_spaces':
        break;
      case 'chat_stream':
        this.appendToLastModelMessage(event.text);
        break;
      case 'log_message':
        this.appendSystemLog(event.level, event.message);
        break;
      case 'flow_event':
        this.handleFlowEvent(event);
        break;
      case 'agent_stream':
        this.activeAgents.update(currentMap => {
          const updatedMap = new Map(currentMap);
          let agent = updatedMap.get(event.agentId);
          if (!agent) {
            agent = { id: event.agentId, instructions: '', stream: '', status: 'running' };
          }
          agent.stream += event.text;
          updatedMap.set(event.agentId, agent);
          return updatedMap;
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

  private handleFlowEvent(event: DomainFlowEvent): void {
    this.flowStates.update(currentFlows => {
      const updatedMap = new Map(currentFlows);
      let flow = updatedMap.get(event.flowId);

      // Inject the flow instantly on start so the inspector is accessible live
      if (event.eventType === 'flow_start') {
        flow = {
          flowId: event.flowId,
          taskId: event.taskId,
          agentCount: event.agentCount,
          completedCount: 0,
          status: 'running',
          agents: new Map()
        };
        updatedMap.set(event.flowId, flow);

        this.coreChat.update(chatHistory => [...chatHistory, { 
          id: crypto.randomUUID(), 
          source: 'flow_card', 
          content: 'Orchestrating Agents...', 
          flowId: event.flowId,
          status: 'running'
        }]);
      }

      if (event.eventType === 'flow_complete' && flow) {
        flow.completedCount++;
        if (flow.completedCount === flow.agentCount && flow.status !== 'completed') {
          flow.status = 'completed';
          
          // Update the live card to completed
          this.coreChat.update(chatHistory => chatHistory.map(item => 
            item.flowId === event.flowId ? { ...item, status: 'completed', content: 'Orchestration Flow Completed' } : item
          ));
        }
      }

      if (!flow) return updatedMap;

      if (event.agentId) {
        const agent = flow.agents.get(event.agentId) || {
          agentId: event.agentId,
          agentIndex: event.agentIndex || 0,
          instruction: '',
          status: 'starting',
          attempt: 1,
          trace: '',
          passed: false,
          timeline: []
        };

        const currentAttempt = event.attempt || agent.attempt;
        if (event.instruction) agent.instruction = event.instruction;

        // Append-only state logging
        if (event.status && event.status !== agent.status) {
          agent.timeline.push({ id: crypto.randomUUID(), timestamp: Date.now(), message: `[Attempt ${currentAttempt}] Status: ${event.status}`, isError: false });
          agent.status = event.status;
        }
        
        // Append-only error logging
        if (event.trace) {
          agent.timeline.push({ id: crypto.randomUUID(), timestamp: Date.now(), message: `[Attempt ${currentAttempt}] Error: ${event.trace}`, isError: true });
          agent.trace = event.trace;
        }

        if (event.attempt) agent.attempt = event.attempt;
        if (event.passed !== undefined) agent.passed = event.passed;

        flow.agents.set(event.agentId, agent);
      }

      return updatedMap;
    });
  }

  private appendCoreChat(source: ChatItem['source'], content: string): void {
    this.coreChat.update(chatHistory => [...chatHistory, { id: crypto.randomUUID(), source, content }]);
  }

  private appendToLastModelMessage(chunkText: string): void {
    this.coreChat.update(chatHistory => {
      if (chatHistory.length === 0 || chatHistory[chatHistory.length - 1].source !== 'model') {
        return [...chatHistory, { id: crypto.randomUUID(), source: 'model', content: chunkText }];
      }
      const updatedChatHistory = [...chatHistory];
      updatedChatHistory[updatedChatHistory.length - 1].content += chunkText;
      return updatedChatHistory;
    });
  }

  private appendSystemLog(level: string, message: string): void {
    this.systemLogs.update(logs => [...logs, { id: crypto.randomUUID(), timestamp: Date.now(), level, message }]);
  }

  public selectStrategy(strategy: DomainDelegationStrategy): void {
    this.pendingStrategy.set(false);
    this.appendCoreChat('user', `Selected Strategy: ${DomainDelegationStrategy[strategy]}`);
    const protocolBufferEvent = LlmFacade.createSelectStrategy(strategy);
    this.transportService.send(protocolBufferEvent);
  }

  public submitReview(accepted: boolean): void {
    const branch = this.pendingReviewBranch();
    if (!branch) return;
    
    this.pendingReviewBranch.set(null);
    this.appendCoreChat('user', `Review for ${branch}: ${accepted ? 'ACCEPTED' : 'REJECTED'}`);
    const protocolBufferEvent = LlmFacade.createReviewDecision(branch, accepted);
    this.transportService.send(protocolBufferEvent);
  }
}