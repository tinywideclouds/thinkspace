import { Injectable, signal, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { TransportService } from '@org/llm-core-transport';
import { LlmFacade, DomainEvent, DomainDelegationStrategy, DomainPreviewContext, DomainTelemetryEvent } from '@org/llm-core-facade';
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
  public pendingContextReview = signal<DomainPreviewContext | null>(null);
  public pendingTransition = signal<{targetDomain: string, reasoning: string} | null>(null);
  
  public autoApproveContext = signal<boolean>(true);
  public retainLogs = signal<boolean>(false);
  public connected = signal<boolean>(false);
  public inspectedReceipt = signal<string | null>(null);

  public connect(url: string): void {
    this.transportService.connect(url).subscribe({
      next: (protocolBufferEvent) => {
        const domainEvent = LlmFacade.toDomain(protocolBufferEvent);
        if (domainEvent) this.handleEvent(domainEvent);
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
    this.pendingContextReview.set(null);
    this.pendingTransition.set(null);
    this.inspectedReceipt.set(null);
  }

  private executeTurnAction(): void {
    if (!this.retainLogs()) {
      this.systemLogs.set([]);
    }
    // Agents are strictly ephemeral; clear them for the new turn.
    this.flowStates.set(new Map());
  }

  public manualClearLogs(): void {
    this.systemLogs.set([]);
  }

  public submitPrompt(text: string): void {
    const spaceId = this.workspaceStateService.activeSpaceId();
    const chatId = this.workspaceStateService.activeChatId();

    if (!spaceId || !chatId) {
      this.appendSystemLog('ERROR', 'Cannot submit prompt: No active space or chat selected.');
      return;
    }

    this.executeTurnAction();
    
    this.appendCoreChat('user', text);
    this.appendCoreChat('system', 'Manager Thinking...');
    
    const protocolBufferEvent = LlmFacade.createSubmitPrompt(text, spaceId, chatId);
    this.transportService.send(protocolBufferEvent);
  }

  public submitContextApproval(approved: boolean, retainedFilePaths: string[] = [], retainedHistoryIndices: number[] = []): void {
    this.executeTurnAction();
    this.pendingContextReview.set(null);
    
    if (!approved) {
      this.appendSystemLog('INFO', 'Turn aborted by user during context review.');
      this.coreChat.update(chatHistory => chatHistory.filter(c => !(c.source === 'system' && c.content.includes('Thinking'))));
    }
    
    const protocolBufferEvent = LlmFacade.createApproveContext(approved, retainedFilePaths, retainedHistoryIndices);
    this.transportService.send(protocolBufferEvent);
  }

  public selectStrategy(strategy: DomainDelegationStrategy): void {
    this.executeTurnAction();
    this.pendingStrategy.set(false);
    this.appendCoreChat('user', `Selected Strategy: ${DomainDelegationStrategy[strategy]}`);
    this.transportService.send(LlmFacade.createSelectStrategy(strategy));
  }

  public submitReview(accepted: boolean): void {
    this.executeTurnAction();
    const branch = this.pendingReviewBranch();
    if (!branch) return;
    
    this.pendingReviewBranch.set(null);
    this.appendCoreChat('user', `Review for ${branch}: ${accepted ? 'ACCEPTED' : 'REJECTED'}`);
    this.transportService.send(LlmFacade.createReviewDecision(branch, accepted));
  }

  public submitTransitionDecision(approved: boolean): void {
    this.executeTurnAction();
    const transition = this.pendingTransition();
    this.pendingTransition.set(null);

    if (approved) {
      this.appendSystemLog('INFO', `Workspace transition to '${transition?.targetDomain}' approved by user.`);
    } else {
      this.appendSystemLog('INFO', 'Workspace transition aborted by user.');
    }

    const protocolBufferEvent = LlmFacade.createTransitionDecision(approved);
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

  public async copyFullContextPreview(previewId: string): Promise<void> {
    const spaceId = this.workspaceStateService.activeSpaceId();
    const chatId = this.workspaceStateService.activeChatId();

    if (!spaceId || !chatId) return;

    try {
      const url = `/api/previews/${encodeURIComponent(chatId)}/${encodeURIComponent(previewId)}?space=${encodeURIComponent(spaceId)}`;
      const markdownData = await firstValueFrom(this.httpClient.get(url, { responseType: 'text' }));
      
      await navigator.clipboard.writeText(markdownData);
      this.appendSystemLog('INFO', `Successfully copied full context payload (${previewId}) to clipboard.`);
      
    } catch (error) {
      console.error(`Failed to fetch preview ${previewId}`, error);
      this.appendSystemLog('ERROR', `Failed to copy full context payload.`);
    }
  }

  private handleEvent(event: DomainEvent): void {
    switch (event.type) {
      case 'preview_context':
        if (this.autoApproveContext()) {
          const filePaths = event.context.fileContexts.map(f => f.path);
          const historyIndices = event.context.recentHistory.map((_, i) => i);
          this.submitContextApproval(true, filePaths, historyIndices);
        } else {
          this.pendingContextReview.set(event.context);
        }
        break;
      case 'chat_stream':
        this.appendToLastModelMessage(event.text);
        break;
      case 'telemetry_event':
        this.handleTelemetryEvent(event);
        break;
      case 'request_strategy':
        this.pendingStrategy.set(event.active);
        break;
      case 'request_review':
        this.pendingReviewBranch.set(event.branch);
        break;
      case 'request_transition':
        this.pendingTransition.set({ targetDomain: event.targetDomain, reasoning: event.reasoning });
        break;
    }
  }

  private handleTelemetryEvent(event: DomainTelemetryEvent): void {
    const payload = event.payload || {};
    const flowId = payload.flow_id;

    if (event.channel === 'manager') {
      if (event.eventType === 'log' || event.eventType === 'error') {
        const level = event.eventType === 'error' ? 'ERROR' : 'INFO';
        this.appendSystemLog(level, event.message);
      }

      if (event.eventType === 'flow_start') {
        this.flowStates.update(currentFlows => {
          const updatedMap = new Map(currentFlows);
          updatedMap.set(flowId, {
            flowId: flowId,
            taskId: payload.task_id || '',
            agentCount: payload.agent_count || 1,
            completedCount: 0,
            status: 'running',
            agents: new Map()
          });
          return updatedMap;
        });

        this.coreChat.update(chatHistory => {
          const cleanHistory = chatHistory.filter(c => !(c.source === 'system' && c.content.includes('Thinking')));
          return [...cleanHistory, { 
            id: crypto.randomUUID(), 
            source: 'flow_card', 
            content: event.summary || 'Orchestrating Agents...', 
            flowId: flowId,
            status: 'running'
          }];
        });
      }

      if (event.eventType === 'flow_complete') {
        this.flowStates.update(currentFlows => {
          const updatedMap = new Map(currentFlows);
          const flow = updatedMap.get(flowId);
          if (flow) {
            flow.completedCount++;
            if (flow.completedCount === flow.agentCount) {
              updatedMap.set(flowId, { ...flow, status: 'completed' });
              this.coreChat.update(chatHistory => chatHistory.map(item => 
                item.flowId === flowId ? { ...item, status: 'completed' } : item
              ));
            } else {
              updatedMap.set(flowId, { ...flow });
            }
          }
          return updatedMap;
        });
      }
    }

    if (flowId) {
      const agentIndex = payload.agent_index;
      if (agentIndex !== undefined) {
        this.flowStates.update(currentFlows => {
          const updatedMap = new Map(currentFlows);
          const flow = updatedMap.get(flowId);
          if (!flow) return updatedMap;

          const updatedAgents = new Map(flow.agents);
          const agentId = event.channel.startsWith('agent-') ? event.channel : `agent-${agentIndex}`;
          
          const existingAgent = updatedAgents.get(agentId);
          const agent: FlowAgentState = existingAgent 
            ? { ...existingAgent, timeline: [...existingAgent.timeline] } 
            : {
              agentId,
              agentIndex,
              instruction: '',
              status: 'starting',
              attempt: 1,
              trace: '',
              passed: false,
              timeline: []
            };

          if (payload.instruction) agent.instruction = payload.instruction;
          if (payload.attempt) agent.attempt = payload.attempt;
          if (payload.passed !== undefined) agent.passed = payload.passed;

          if (event.eventType === 'flow_error') {
            agent.trace = payload.trace || event.message;
            agent.status = 'failed';
            agent.timeline.push({ id: crypto.randomUUID(), timestamp: Date.now(), message: `[Attempt ${agent.attempt}] Error: ${agent.trace}`, isError: true });
          } else if (event.eventType === 'flow_status') {
            const statusMsg = event.message || 'working';
            if (agent.status !== statusMsg) {
              agent.status = statusMsg;
              agent.timeline.push({ id: crypto.randomUUID(), timestamp: Date.now(), message: `[Attempt ${agent.attempt}] Status: ${statusMsg}`, isError: false });
            }
          } else if (event.eventType === 'flow_complete') {
            agent.status = payload.passed ? 'passed' : 'failed';
            agent.timeline.push({ id: crypto.randomUUID(), timestamp: Date.now(), message: `[Attempt ${agent.attempt}] Completed: ${agent.status}`, isError: !payload.passed });
          }

          updatedAgents.set(agentId, agent);
          updatedMap.set(flowId, { ...flow, agents: updatedAgents });
          
          return updatedMap;
        });
      }
    }
  }

  private appendCoreChat(source: ChatItem['source'], content: string): void {
    this.coreChat.update(chatHistory => {
      const cleanHistory = chatHistory.filter(c => !(c.source === 'system' && c.content.includes('Thinking')));
      return [...cleanHistory, { id: crypto.randomUUID(), source, content }];
    });
  }

  private appendToLastModelMessage(chunkText: string): void {
    this.coreChat.update(chatHistory => {
      const cleanHistory = chatHistory.filter(c => !(c.source === 'system' && c.content.includes('Thinking')));
      
      if (cleanHistory.length === 0 || cleanHistory[cleanHistory.length - 1].source !== 'model') {
        return [...cleanHistory, { id: crypto.randomUUID(), source: 'model', content: chunkText }];
      }
      
      const updatedChatHistory = [...cleanHistory];
      updatedChatHistory[updatedChatHistory.length - 1].content += chunkText;
      return updatedChatHistory;
    });
  }

  private appendSystemLog(level: string, message: string): void {
    this.systemLogs.update(logs => [...logs, { id: crypto.randomUUID(), timestamp: Date.now(), level, message }]);
  }
}