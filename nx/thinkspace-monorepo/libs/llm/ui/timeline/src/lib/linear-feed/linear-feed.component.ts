import { Component, input, output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ChatItem, FlowState } from '@org/llm-state-chat';
// Note: You will need to move MarkdownComponent and ChatFlowCardComponent 
// to a shared ui library or update their imports based on your exact Nx boundaries.
import { ChatFlowCardComponent, MarkdownComponent } from '@org/llm-ui-chat';

@Component({
  selector: 'llm-linear-feed',
  standalone: true,
  imports: [CommonModule, ChatFlowCardComponent, MarkdownComponent],
  templateUrl: './linear-feed.component.html',
  styleUrl: './linear-feed.component.scss'
})
export class LinearFeedComponent {
  feed = input.required<ChatItem[]>();
  flowStates = input.required<Map<string, FlowState>>();
  inspectFlow = output<string>();
}