import { Component, input, output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ChatItem, FlowState } from '@org/llm-state-chat';
import { ChatFlowCardComponent } from '../chat-flow-card/chat-flow-card.component';
import { MarkdownPipe } from '../pipes/markdown.pipe';

@Component({
  selector: 'llm-chat-feed',
  standalone: true,
  imports: [CommonModule, ChatFlowCardComponent, MarkdownPipe],
  template: `
    <div style="display: flex; flex-direction: column; gap: 12px;">
      @for (item of feed(); track item.id) {
        @if (item.source === 'flow_card' && item.flowId) {
          <llm-chat-flow-card 
            [flow]="flowStates().get(item.flowId)" 
            [summary]="item.content"
            (inspect)="inspectFlow.emit($event)">
          </llm-chat-flow-card>
        } @else if (item.source === 'model' || item.source === 'system') {
          <div style="padding: 12px; border-radius: 8px; background: #f8f9fa; border: 1px solid #dee2e6;">
            <strong style="text-transform: uppercase; font-size: 0.75em; color: #868e96; letter-spacing: 0.5px;">{{ item.source }}</strong>
            <div [innerHTML]="item.content | markdown" style="margin-top: 8px; white-space: pre-wrap; font-size: 14px; color: #212529; line-height: 1.5;"></div>
          </div>
        } @else {
          <div style="padding: 12px; border-radius: 8px; background: #e7f5ff; border: 1px solid #74c0fc;">
            <strong style="text-transform: uppercase; font-size: 0.75em; color: #1864ab; letter-spacing: 0.5px;">{{ item.source }}</strong>
            <div style="margin-top: 8px; white-space: pre-wrap; font-size: 14px; color: #1864ab;">{{ item.content }}</div>
          </div>
        }
      }
    </div>
  `
})
export class ChatFeedComponent {
  feed = input.required<ChatItem[]>();
  flowStates = input.required<Map<string, FlowState>>();
  inspectFlow = output<string>();
}