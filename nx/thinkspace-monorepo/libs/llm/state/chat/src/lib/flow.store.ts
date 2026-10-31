import { Injectable, signal } from '@angular/core';
import { TelemetryEvent } from '@org/llm-core-facade';
import { FlowState, LogItem, TimelineEntry } from './chat.models';

@Injectable({ providedIn: 'root' })
export class FlowStore {
  public flowStates = signal<Map<string, FlowState>>(new Map());
  public systemLogs = signal<LogItem[]>([]);
  public retainLogs = signal<boolean>(false);
  public inspectedReceipt = signal<string | null>(null);

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
      
      const flowId = event.payload['flow_id'] as string;
      if (flowId) {
        if (event.eventType === 'flow_start') {
           const taskId = event.payload['task_id'] as string || 'unknown';
           const agentCount = (event.payload['agent_count'] as number) || 0;
           this.flowStates.update(map => {
             const newMap = new Map(map);
             newMap.set(flowId, {
               flowId,
               taskId,
               agentCount,
               completedCount: 0,
               status: 'running',
               agents: new Map()
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
    const flowId = event.payload['flow_id'] as string;
    const agentId = event.payload['agent_id'] as string;
    if (!flowId || !agentId) return;

    this.flowStates.update(map => {
      const newMap = new Map(map);
      const flow = newMap.get(flowId);
      if (!flow) return newMap;

      const agents = new Map(flow.agents);
      let agent = agents.get(agentId);

      if (!agent) {
        agent = {
          agentId,
          agentIndex: (event.payload['agent_index'] as number) || 0,
          instruction: (event.payload['instruction'] as string) || '',
          status: 'running',
          attempt: 1,
          trace: '',
          passed: false,
          timeline: []
        };
      }

      const timelineEntry: TimelineEntry = {
        id: event.id || crypto.randomUUID(),
        timestamp: Date.now(), 
        message: event.message || event.eventType,
        isError: event.eventType.includes('error') || event.eventType.includes('failed')
      };

      if (event.eventType === 'agent_status') {
         agent.status = (event.payload['status'] as string) || agent.status;
      } else if (event.eventType === 'agent_complete') {
         agent.status = 'completed';
         agent.passed = !!event.payload['passed'];
         agent.trace = (event.payload['trace'] as string) || '';
      } else if (event.eventType === 'agent_error') {
         agent.status = 'error';
         agent.passed = false;
         agent.trace = (event.payload['trace'] as string) || '';
      }

      agent.timeline = [...agent.timeline, timelineEntry];
      agents.set(agentId, agent);
      newMap.set(flowId, { ...flow, agents });

      return newMap;
    });
  }

  public inspectFlow(flowId: string): void {
    console.log(`[FlowStore] Inspecting flow ${flowId}`);
    const mockReceipt = `<?xml version="1.0" encoding="UTF-8"?>
    <FlowReceipt FlowID="${flowId}" TaskID="task-1" Timestamp="${new Date().toISOString()}">
      <Task>Inspected Task</Task>
      <Summary>Flow execution summary for ${flowId}</Summary>
      <Agents>
        <Agent AgentID="agent-1" Passed="true">
          <Instruction>Execute flow tasks based on provided instructions.</Instruction>
          <RawPayload>{}</RawPayload>
          <VerificationTrace>[VERIFY] check dependencies: SUCCESS</VerificationTrace>
          <StateDelta></StateDelta>
        </Agent>
      </Agents>
    </FlowReceipt>`;
    this.inspectedReceipt.set(mockReceipt);
  }

  public resetTurn(): void {
    if (!this.retainLogs()) {
      this.systemLogs.set([]);
    }
  }

  public clearSession(): void {
    this.flowStates.set(new Map());
    this.systemLogs.set([]);
  }
}