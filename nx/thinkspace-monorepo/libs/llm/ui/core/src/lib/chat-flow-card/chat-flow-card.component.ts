import { Component, input, output, computed } from '@angular/core';
import { FlowState, FlowExecutionChatItem } from '@org/llm-state-chat';
import { AgentRecord } from '@org/llm-core-facade';

export interface DisplayAgent extends AgentRecord {
  status?: string;
}

@Component({
  selector: 'llm-chat-flow-card',
  standalone: true,
  template: `
    <div style="background: #e9ecef; border: 1px solid #ced4da; border-radius: 8px; padding: 12px; margin: 8px 0;">
      
      <!-- Immutable Task Header -->
      <div style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 12px;">
        <div style="color: #212529; font-size: 14px; font-weight: bold; flex: 1; margin-right: 16px;">
          Task: {{ item().task }}
        </div>
        
        <div style="display: flex; gap: 8px;">
          <button
            (click)="viewGraph.emit(item().flowId)"
            style="background: #e7f5ff; border: 1px solid #74c0fc; padding: 6px 12px; border-radius: 4px; cursor: pointer; font-size: 12px; font-weight: bold; color: #1864ab; white-space: nowrap;"
          >
            🕸️ View Graph
          </button>
          <button
            (click)="inspect.emit(item())"
            style="background: #fff; border: 1px solid #adb5bd; padding: 6px 12px; border-radius: 4px; cursor: pointer; font-size: 12px; font-weight: bold; color: #495057; white-space: nowrap;"
          >
            🔍 Inspect Details
          </button>
        </div>
      </div>

      <!-- Live Ephemeral Agent State Overlay -->
      @if (displayStatus() === 'RUNNING' || (displayStatus() === 'FAILED' && item().status !== 'FAILED')) {
        <div style="display: flex; flex-direction: column; gap: 4px; padding: 8px; background: #fff; border-radius: 4px; border: 1px dashed #adb5bd; margin-bottom: 12px;">
          @for (agent of displayAgents(); track agent.agentId) {
            <div style="font-size: 12px; color: #495057; font-family: monospace;">
              <span style="font-weight: bold;">{{ agent.agentId }}:</span>
              <span style="text-transform: capitalize;">
                {{ agent.status ? agent.status.split('_').join(' ') : (agent.passed ? 'Passed' : 'Failed') }}
              </span>
            </div>
          }
        </div>
      }

      <!-- Mutable Status & Final Result Body -->
      <div style="display: flex; align-items: flex-start;">
        <strong style="color: #495057; display: block; font-size: 13px;">
          {{ displayStatus() === 'RUNNING' ? '⏳ Status:' : displayStatus() === 'FAILED' ? '❌ Result:' : '✅ Result:' }} 
          <span style="font-weight: normal;">{{ item().content }}</span>
        </strong>
      </div>

      <!-- Detailed Semantic Error (Final State Only) -->
      @if (item().status === 'FAILED' && item().error) {
        <div style="margin-top: 12px; padding: 8px 12px; background: #fff5f5; border: 1px solid #ffc9c9; border-radius: 4px; border-left: 4px solid #fa5252;">
          <strong style="display: block; font-size: 11px; color: #e03131; text-transform: uppercase; margin-bottom: 4px;">Root Cause Analysis</strong>
          <span style="font-size: 13px; color: #c92a2a;">{{ item().error }}</span>
        </div>
      }
    </div>
  `,
})
export class ChatFlowCardComponent {
  public item = input.required<FlowExecutionChatItem>();
  public liveFlow = input<FlowState | undefined>();
  
  public inspect = output<FlowExecutionChatItem>();
  public viewGraph = output<string>();

  public isLive = computed(() => {
    return this.item().status === 'RUNNING' && this.liveFlow() !== undefined;
  });

  public displayStatus = computed(() => {
    const live = this.liveFlow();
    return this.isLive() && live ? live.status.toUpperCase() : this.item().status;
  });

  public displayAgents = computed<DisplayAgent[]>(() => {
    const live = this.liveFlow();
    if (this.isLive() && live) {
      return Array.from(live.agents.values()).sort(
        (a, b) => a.agentIndex - b.agentIndex
      ).map(a => ({
        agentId: a.agentId,
        passed: a.passed,
        instruction: a.instruction,
        rawPayload: '',
        verificationTrace: a.trace,
        stateDelta: '',
        status: a.status
      }));
    }
    return this.item().agents;
  });
}