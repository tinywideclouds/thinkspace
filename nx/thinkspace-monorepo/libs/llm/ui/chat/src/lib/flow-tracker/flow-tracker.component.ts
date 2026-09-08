import { Component, input, computed } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FlowState } from '@org/llm-state-chat';

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
}