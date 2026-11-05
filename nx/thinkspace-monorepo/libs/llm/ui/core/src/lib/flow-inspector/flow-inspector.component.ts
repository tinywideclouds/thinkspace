import { Component, input, output, computed, signal, effect } from '@angular/core';
import { CommonModule } from '@angular/common';
import { parseVerificationTrace, TraceStep, AgentRecord } from '@org/llm-core-facade';
import { FlowExecutionChatItem } from '@org/llm-state-chat';

@Component({
  selector: 'llm-flow-inspector',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './flow-inspector.component.html',
})
export class FlowInspectorComponent {
  public flowItem = input.required<FlowExecutionChatItem>();
  public inspectorClosed = output<void>();

  public selectedAgentId = signal<string | null>(null);

  constructor() {
    effect(() => {
      const item = this.flowItem();
      if (item && item.agents && item.agents.length > 0 && !this.selectedAgentId()) {
        this.selectedAgentId.set(item.agents[0].agentId);
      }
    });
  }

  public activeAgent = computed<AgentRecord | null>(() => {
    const item = this.flowItem();
    const id = this.selectedAgentId();
    if (!item || !item.agents || !id) return null;
    return item.agents.find((a) => a.agentId === id) || null;
  });

  public parsedTraceSteps = computed<TraceStep[]>(() => {
    const agent = this.activeAgent();
    if (!agent || !agent.verificationTrace) return [];
    return parseVerificationTrace(agent.verificationTrace);
  });
}