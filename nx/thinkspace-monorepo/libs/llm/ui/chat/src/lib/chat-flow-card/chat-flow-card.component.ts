import { Component, input, output, computed } from '@angular/core';
import { FlowState } from '@org/llm-state-chat';

@Component({
  selector: 'llm-chat-flow-card',
  standalone: true,
  template: `
    <div style="background: #e9ecef; border: 1px solid #ced4da; border-radius: 8px; padding: 12px; margin: 8px 0;">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px;">
        <div>
          <strong style="color: #495057; display: block; font-size: 14px;">
            {{ flow()?.status === 'running' ? '⏳' : '✅' }} {{ summary() }} {{ flow()?.status === 'completed' ? '(Completed)' : '' }}
          </strong>
        </div>
        <button 
          (click)="inspect.emit(flow()?.flowId!)"
          style="background: #fff; border: 1px solid #adb5bd; padding: 6px 12px; border-radius: 4px; cursor: pointer; font-size: 12px; font-weight: bold; color: #495057;">
          🔍 Inspect Details
        </button>
      </div>
      
      @if (flow()?.status === 'running') {
        <div style="display: flex; flex-direction: column; gap: 4px; padding: 8px; background: #fff; border-radius: 4px; border: 1px dashed #adb5bd;">
          @for (agent of agentsArray(); track agent.agentId) {
            <div style="font-size: 12px; color: #495057; font-family: monospace;">
              <span style="font-weight: bold;">Agent {{ agent.agentIndex }}:</span> 
              <span style="text-transform: capitalize;">{{ agent.status.split('_').join(' ') }}</span>
            </div>
          }
        </div>
      }
    </div>
  `
})
export class ChatFlowCardComponent {
  flow = input.required<FlowState | undefined>();
  summary = input<string>('Orchestration Flow');
  inspect = output<string>();

  agentsArray = computed(() => {
    const f = this.flow();
    if (!f) return [];
    return Array.from(f.agents.values()).sort((a, b) => a.agentIndex - b.agentIndex);
  });
}