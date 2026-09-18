import { Component, input, computed, signal, effect, output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FlowState, LogItem } from '@org/llm-state-chat';

@Component({
  selector: 'llm-flow-tracker',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './flow-tracker.component.html'
})
export class FlowTrackerComponent {
  systemLogs = input.required<LogItem[]>();
  flows = input.required<Map<string, FlowState>>();
  retainLogs = input.required<boolean>();

  toggleRetainLogs = output<void>();
  manualClearLogs = output<void>();

  activeTab = signal<string>('manager');

  constructor() {
    // If the active agent is purged from state, gracefully fall back to the manager logs
    effect(() => {
      if (this.activeTab() !== 'manager' && !this.activeAgentData()) {
        this.activeTab.set('manager');
      }
    });
  }

  flowsArray = computed(() => {
    return Array.from(this.flows().values())
      .map(flow => ({
        ...flow,
        agentsArray: Array.from(flow.agents.values()).sort((a, b) => a.agentIndex - b.agentIndex)
      }))
      .sort((a, b) => a.status === 'running' ? -1 : 1); 
  });

  activeAgentData = computed(() => {
    const targetId = this.activeTab();
    if (targetId === 'manager') return null;
    for (const flow of this.flowsArray()) {
      const found = flow.agentsArray.find(a => a.agentId === targetId);
      if (found) return found;
    }
    return null;
  });
}