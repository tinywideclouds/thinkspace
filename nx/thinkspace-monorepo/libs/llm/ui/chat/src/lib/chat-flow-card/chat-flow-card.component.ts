import { Component, input, output } from '@angular/core';

@Component({
  selector: 'llm-chat-flow-card',
  standalone: true,
  template: `
    <div style="background: #e9ecef; border: 1px solid #ced4da; border-radius: 8px; padding: 12px; margin: 8px 0; display: flex; justify-content: space-between; align-items: center;">
      <div>
        <strong style="color: #495057; display: block; font-size: 14px;">⚙️ Orchestration Flow</strong>
        <span style="color: #868e96; font-size: 12px; font-family: monospace;">{{ flowId() }}</span>
      </div>
      <button 
        (click)="inspect.emit(flowId())"
        style="background: #fff; border: 1px solid #adb5bd; padding: 6px 12px; border-radius: 4px; cursor: pointer; font-size: 12px; font-weight: bold; color: #495057; transition: background 0.2s;"
        onmouseover="this.style.background='#f8f9fa'" 
        onmouseout="this.style.background='#fff'">
        🔍 Inspect Details
      </button>
    </div>
  `
})
export class ChatFlowCardComponent {
  flowId = input.required<string>();
  inspect = output<string>();
}