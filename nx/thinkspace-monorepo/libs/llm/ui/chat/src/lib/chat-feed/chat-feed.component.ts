import { Component, input, output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ChatItem } from '@org/llm-state-chat';
import { ChatFlowCardComponent } from '../chat-flow-card/chat-flow-card.component';

@Component({
  selector: 'llm-chat-feed',
  standalone: true,
  imports: [CommonModule, ChatFlowCardComponent],
  template: `
    <div style="display: flex; flex-direction: column; gap: 8px;">
      @for (item of feed(); track item.id) {
        @if (item.source === 'flow_card' && item.flowId) {
          <llm-chat-flow-card 
            [flowId]="item.flowId" 
            [status]="item.status || 'completed'"
            (inspect)="inspectFlow.emit($event)">
          </llm-chat-flow-card>
        } @else {
          <div style="padding: 8px; border-radius: 4px; background: #f0f0f0;">
            <strong style="text-transform: uppercase; font-size: 0.8em; color: #555;">{{ item.source }}</strong>
            <pre style="margin: 4px 0 0; white-space: pre-wrap; font-family: monospace;">{{ item.content }}</pre>
          </div>
        }
      }
    </div>
  `
})
export class ChatFeedComponent {
  feed = input.required<ChatItem[]>();
  inspectFlow = output<string>();
}