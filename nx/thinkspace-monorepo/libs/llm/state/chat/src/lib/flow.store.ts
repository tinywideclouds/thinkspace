import { Injectable, signal } from '@angular/core';
import { TelemetryEvent } from '@org/llm-core-facade';
import { FlowState, LogItem, TimelineEntry, FlowAgentState, FlowExecutionChatItem } from './chat.models';

@Injectable({ providedIn: 'root' })
export class FlowStore {
  public flowStates = signal(new Map<string, FlowState>());
  public systemLogs = signal<LogItem[]>([]);
  public retainLogs = signal(false);
  
  public inspectedFlow = signal<FlowExecutionChatItem | null>(null);
  
  // ADDED: The signal driving the full-screen DAG overlay
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
    const payload = event.payload || {};

    if (event.channel === 'manager') {
      this.appendSystemLog('INFO', event.message || event.summary || event.eventType);
      
      const flowId = typeof payload['flow_id'] === 'string' ? payload['flow_id'] : '';
      if (flowId) {
        if (event.eventType === 'flow_start') {
           const taskId = typeof payload['task_id'] === 'string' ? payload['task_id'] : 'unknown';
           const agentCount = typeof payload['agent_count'] === 'number' ? payload['agent_count'] : 0;
           
           this.flowStates.update(map => {
             const newMap = new Map(map);
             newMap.set(flowId, {
               flowId,
               taskId,
               agentCount,
               completedCount: 0,
               status: 'running',
               agents: new Map<string, FlowAgentState>()
             });
             return newMap;
           });
        } else if (event.eventType === 'flow_complete') {
           this.flowStates.update(map => {
             const newMap = new Map(map);
             const flow = newMap.get(flowId);
             if (flow) {
               newMap.set(flowId, { ...flow, status: 'completed' });
             }
             return newMap;
           });
        }
      }
      return;
    }

    // Agent channel telemetry
    const flowId = typeof payload['flow_id'] === 'string' ? payload['flow_id'] : '';
    const agentId = typeof payload['agent_id'] === 'string' ? payload['agent_id'] : '';
    if (!flowId || !agentId) return;

    this.flowStates.update(map => {
      const newMap = new Map(map);
      const flow = newMap.get(flowId);
      if (!flow) return newMap;

      const agents = new Map(flow.agents);
      const existingAgent = agents.get(agentId);

      const agentIndex = typeof payload['agent_index'] === 'number' ? payload['agent_index'] : 0;
      const instruction = typeof payload['instruction'] === 'string' ? payload['instruction'] : '';

      const agent: FlowAgentState = existingAgent || {
        agentId,
        agentIndex,
        instruction,
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

      if (event.eventType === 'agent_status') {
         agent.status = typeof payload['status'] === 'string' ? payload['status'] : agent.status;
      } else if (event.eventType === 'agent_complete') {
         agent.status = 'completed';
         agent.passed = Boolean(payload['passed']);
         agent.trace = typeof payload['trace'] === 'string' ? payload['trace'] : '';
      } else if (event.eventType === 'agent_error') {
         agent.status = 'error';
         agent.passed = false;
         agent.trace = typeof payload['trace'] === 'string' ? payload['trace'] : '';
      }

      agent.timeline = [...agent.timeline, timelineEntry];
      agents.set(agentId, agent);
      newMap.set(flowId, { ...flow, agents });

      return newMap;
    });
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