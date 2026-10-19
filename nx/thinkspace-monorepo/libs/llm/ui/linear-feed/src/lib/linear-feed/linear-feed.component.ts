import { Component, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ChatFlowCardComponent, MarkdownComponent } from '@org/llm-ui-core';
import { LinearFeedStore } from '../linear-feed.store';

@Component({
  selector: 'llm-linear-feed',
  standalone: true,
  imports: [CommonModule, ChatFlowCardComponent, MarkdownComponent],
  template: `
    <div class="flex flex-col gap-3">
      @for (item of store.visibleFeed(); track item.id) {
        
        @if (item.source === 'flow_card' && item.flowId) {
          <llm-chat-flow-card 
            [flow]="store.flowStates().get(item.flowId)" 
            [summary]="item.content"
            (inspect)="store.inspectFlow($event)">
          </llm-chat-flow-card>
        } 
        
        @else if (item.source === 'model' || item.source === 'system') {
          <div class="p-3 rounded-lg bg-gray-50 border border-gray-200">
            <strong class="block mb-2 text-xs font-bold uppercase tracking-wide text-gray-500">
              {{ item.source }}
            </strong>
            <llm-markdown [content]="item.content"></llm-markdown>
          </div>
        } 
        
        @else {
          <div class="p-3 rounded-lg bg-blue-50 border border-blue-300">
            <strong class="block mb-2 text-xs font-bold uppercase tracking-wide text-blue-800">
              {{ item.source }}
            </strong>
            <div class="whitespace-pre-wrap text-sm text-blue-800">
              {{ item.content }}
            </div>
          </div>
        }
      }
    </div>
  `
})
export class LinearFeedComponent {
  public store = inject(LinearFeedStore);
}