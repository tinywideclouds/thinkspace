import { Component, input, computed } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FlowState, FlowAgentState } from '@org/llm-state-chat';

@Component({
  selector: 'llm-flow-tracker',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './flow-tracker.component.html'
})
export class FlowTrackerComponent {
  flows = input.required<Map<string, FlowState>>();

  flowsArray = computed(() => {
    return Array.from(this.flows().values())
      .map(flow => ({
        ...flow,
        agentsArray: Array.from(flow.agents.values()).sort((a, b) => a.agentIndex - b.agentIndex)
      }))
      .sort((a, b) => a.status === 'running' ? -1 : 1); 
  });

  getAgentColor(agent: FlowAgentState): string {
    if (agent.passed) return '#2b8a3e'; 
    if (agent.status === 'running_tests' || agent.status === 'executing_instructions') return '#339af0'; 
    if (agent.trace) return '#e03131'; 
    return '#868e96'; 
  }

  getAgentIcon(agent: FlowAgentState): string {
    if (agent.passed) return '✅';
    if (agent.status === 'running_tests' || agent.status === 'executing_instructions') return '⚙️';
    if (agent.trace) return '❌';
    return '⏳';
  }

  formatStatus(status: string): string {
    return status.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase());
  }
}