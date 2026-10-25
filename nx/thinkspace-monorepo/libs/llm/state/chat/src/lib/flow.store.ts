import { Injectable, signal, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { ThinkspaceStateService } from './thinkspace-state.service';
import { DomainTelemetryEvent } from '@org/llm-core-facade';
import { firstValueFrom } from 'rxjs';
import { LogItem, FlowState, FlowAgentState } from './chat.models';

@Injectable({ providedIn: 'root' })
export class FlowStore {
  private httpClient = inject(HttpClient);
  private workspaceState = inject(ThinkspaceStateService);

  public flowStates = signal<Map<string, FlowState>>(new Map());
  public systemLogs = signal<LogItem[]>([]);
  public retainLogs = signal<boolean>(false);
  public inspectedReceipt = signal<string | null>(null);

  public resetTurn(): void {
    if (!this.retainLogs()) {
      this.systemLogs.set([]);
    }
    this.flowStates.set(new Map());
  }

  public clearSession(): void {
    this.flowStates.set(new Map());
    this.systemLogs.set([]);
    this.inspectedReceipt.set(null);
  }

  public manualClearLogs(): void {
    this.systemLogs.set([]);
  }

  public appendSystemLog(level: string, message: string): void {
    this.systemLogs.update(logs => [...logs, { id: crypto.randomUUID(), timestamp: Date.now(), level, message }]);
  }

  public async inspectFlow(flowId: string): Promise<void> {
    const spaceId = this.workspaceState.activeSpaceId();
    const chatId = this.workspaceState.activeChatId();
    if (!spaceId || !chatId) return;

    try {
      const url = `/api/receipts/${encodeURIComponent(chatId)}/${encodeURIComponent(flowId)}?space=${encodeURIComponent(spaceId)}`;
      const xmlData = await firstValueFrom(this.httpClient.get(url, { responseType: 'text' }));
      this.inspectedReceipt.set(xmlData);
    } catch (error) {
      this.appendSystemLog('ERROR', `Failed to load receipt for flow ${flowId}`);
    }
  }

  public processTelemetry(event: DomainTelemetryEvent): void {
    const payload = event.payload || {};
    const flowId = payload.flow_id;

    if (event.channel === 'manager') {
      if (event.eventType === 'log' || event.eventType === 'error') {
        this.appendSystemLog(event.eventType === 'error' ? 'ERROR' : 'INFO', event.message);
      }

      if (event.eventType === 'flow_start') {
        this.flowStates.update(current => {
          const updated = new Map(current);
          updated.set(flowId, { flowId, taskId: payload.task_id || '', agentCount: payload.agent_count || 1, completedCount: 0, status: 'running', agents: new Map() });
          return updated;
        });
      }

      if (event.eventType === 'flow_complete') {
        this.flowStates.update(current => {
          const updated = new Map(current);
          const flow = updated.get(flowId);
          if (flow) {
            flow.completedCount++;
            if (flow.completedCount === flow.agentCount) {
              updated.set(flowId, { ...flow, status: 'completed' });
            }
          }
          return updated;
        });
      }
    }

    if (flowId && payload.agent_index !== undefined) {
      this.flowStates.update(currentFlows => {
        const updatedMap = new Map(currentFlows);
        const flow = updatedMap.get(flowId);
        if (!flow) return updatedMap;

        const updatedAgents = new Map(flow.agents);
        const agentId = event.channel.startsWith('agent-') ? event.channel : `agent-${payload.agent_index}`;
        
        const existing = updatedAgents.get(agentId);
        const agent: FlowAgentState = existing ? { ...existing, timeline: [...existing.timeline] } : { agentId, agentIndex: payload.agent_index, instruction: '', status: 'starting', attempt: 1, trace: '', passed: false, timeline: [] };

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