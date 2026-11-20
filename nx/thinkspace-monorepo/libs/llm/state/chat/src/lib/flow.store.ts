import { Injectable, signal } from '@angular/core';
import { TelemetryEvent } from '@org/llm-core-facade';
import { FlowState, LogItem, TimelineEntry, FlowAgentState, FlowExecutionChatItem } from './chat.models';

@Injectable({ providedIn: 'root' })
export class FlowStore {
  public flowStates = signal(new Map<string, FlowState>());
  public systemLogs = signal<LogItem[]>([]);
  public retainLogs = signal(false);
  
  public inspectedFlow = signal<FlowExecutionChatItem | null>(null);
  
  public inspectedGraphId = signal<string | null>(null);

  public appendSystemLog(level: string, message: string): void {
    this.systemLogs.update(logs => [
      ...logs,
      {
        id: crypto.randomUUID(),
        timestamp: Date.now(),
        level,
        message
      }
    ]);
  }

  public manualClearLogs(): void {
    this.systemLogs.set([]);
  }

  public processTelemetry(event: TelemetryEvent): void {
    if (event.channel === 'manager') {
      this.appendSystemLog('INFO', event.message || event.summary || event.eventType);
      
      if (event.payload?.type === 'flow') {
        const payload = event.payload;
        if (event.eventType === 'flow_start') {
           this.flowStates.update(map => {
             const newMap = new Map(map);
             newMap.set(payload.flowId, {
               flowId: payload.flowId,
               taskId: payload.taskId,
               agentCount: payload.agentCount,
               completedCount: 0,
               status: 'running',
               agents: new Map<string, FlowAgentState>()
             });
             return newMap;
           });
        } else if (event.eventType === 'flow_complete') {
           this.flowStates.update(map => {
             const newMap = new Map(map);
             const flow = newMap.get(payload.flowId);
             if (flow) {
               newMap.set(payload.flowId, { ...flow, status: 'completed' });
             }
             return newMap;
           });
        }
        return;
      }
    }

    if (event.payload?.type === 'agent') {
      const payload = event.payload;
      
      this.flowStates.update(map => {
        const newMap = new Map(map);
        const flow = newMap.get(payload.flowId);
        if (!flow) return newMap;

        const agents = new Map(flow.agents);
        const existingAgent = agents.get(payload.agentId);

        const agent: FlowAgentState = existingAgent || {
          agentId: payload.agentId,
          agentIndex: payload.agentIndex,
          instruction: payload.instruction,
          status: 'running',
          attempt: 1,
          trace: '',
          passed: false,
          timeline: []
        };

        const timelineEntry: TimelineEntry = {
          id: event.id || crypto.randomUUID(),
          timestamp: Date.now(), 
          message: event.message || event.eventType,
          isError: event.eventType.includes('error') || event.eventType.includes('failed')
        };

        if (event.eventType === 'agent_status' || event.eventType === 'flow_status') {
          agent.status = event.message || 'running';
        } else if (event.eventType === 'agent_complete' || event.eventType === 'flow_complete') {
          agent.status = payload.passed ? 'completed' : 'failed';
          agent.passed = payload.passed;
          agent.trace = payload.trace;
        } else if (event.eventType === 'agent_error' || event.eventType === 'flow_error') {
          agent.status = event.message || 'verification failed';
          agent.passed = false;
          agent.trace = payload.trace;
        }

        agent.timeline = [...agent.timeline, timelineEntry];
        agents.set(payload.agentId, agent);
        newMap.set(payload.flowId, { ...flow, agents });

        return newMap;
      });
    }
  }

  public inspectFlow(item: FlowExecutionChatItem): void {
    console.log(`[FlowStore] Inspecting flow ${item.flowId}`);
    this.inspectedFlow.set(item);
  }

  public resetTurn(): void {
    if (!this.retainLogs()) {
      this.systemLogs.set([]);
    }
  }

  public clearSession(): void {
    this.flowStates.set(new Map<string, FlowState>());
    this.systemLogs.set([]);
  }
}