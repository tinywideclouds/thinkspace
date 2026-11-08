import { Component, input, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { 
  ChatItem, 
  FlowExecutionChatItem, 
  FileDeltaChatItem, 
  MessageChatItem 
} from '@org/llm-state-chat';
import { TimelineFacade, EnrichedChatItem } from '../timeline.facade';
import { UserMessageBlockComponent } from './user-message-block.component';
import { ModelMessageBlockComponent } from './model-message-block.component';
import { FileDeltaBlockComponent } from './file-delta-block/file-delta-block.component';
import { ChatFlowCardComponent } from '@org/llm-ui-core';

@Component({
  selector: 'llm-block-router',
  standalone: true,
  imports: [
    CommonModule, 
    UserMessageBlockComponent, 
    ModelMessageBlockComponent, 
    FileDeltaBlockComponent,
    ChatFlowCardComponent
  ],
  template: `
    <div class="w-full transition-all duration-300">
      @switch (chatItem().type) {
        
        @case ('flow_execution') {
          @let item = asFlowExecution(chatItem());
          <div [class.opacity-30]="facade.isItemDimmed(item)" [class.h-2]="facade.isItemCollapsed(item)" class="transition-all duration-300 overflow-hidden">
            @if (!facade.isItemCollapsed(item)) {
              <llm-chat-flow-card 
                [item]="item"
                [liveFlow]="facade.flowStates().get(item.flowId)" 
                (inspect)="facade.inspectFlow($event)"
                (viewGraph)="facade.viewGraph($event)">
              </llm-chat-flow-card>
            }
          </div>
        }

        @case ('model') {
          @let item = asEnrichedModel(chatItem());
          <llm-model-message-block 
            [source]="item.type" 
            [content]="item.content"
            [tags]="item.tags || []"
            [attachments]="item.attachments || []"
            [isDimmed]="facade.isItemDimmed(item)"
            [isCollapsed]="facade.isItemCollapsed(item)"
            [activeTag]="facade.hoveredTag() || facade.selectedTag()"
            (tagHover)="facade.setHoveredTag($event)"
            (tagClick)="facade.toggleSelectedTag($event)">
          </llm-model-message-block>
        }

        @case ('system') {
          @let item = asEnrichedModel(chatItem());
          <llm-model-message-block 
            [source]="item.type" 
            [content]="item.content"
            [tags]="item.tags || []"
            [attachments]="item.attachments || []"
            [isDimmed]="facade.isItemDimmed(item)"
            [isCollapsed]="facade.isItemCollapsed(item)"
            [activeTag]="facade.hoveredTag() || facade.selectedTag()"
            (tagHover)="facade.setHoveredTag($event)"
            (tagClick)="facade.toggleSelectedTag($event)">
          </llm-model-message-block>
        }

        @case ('file_delta') {
          @let item = asFileDelta(chatItem());
          <div [class.opacity-30]="facade.isItemDimmed(item)" [class.h-2]="facade.isItemCollapsed(item)" class="transition-all duration-300 overflow-hidden">
            @if (!facade.isItemCollapsed(item)) {
              <llm-file-delta-block
                [path]="item.path"
                [summary]="item.content"
                [previewSnippet]="item.previewSnippet"
                [fileRef]="item.fileRef">
              </llm-file-delta-block>
            }
          </div>
        }

        @case ('prompt') {
          @let item = asPrompt(chatItem());
          <div class="relative transition-all duration-300"
            [ngClass]="{
              'opacity-30 scale-[0.98]': facade.isItemDimmed(item) && !facade.isItemCollapsed(item),
              'h-2 opacity-40 py-0 mb-2 bg-blue-100 border-transparent overflow-hidden': facade.isItemCollapsed(item)
            }">
            @if (!facade.isItemCollapsed(item)) {
              <llm-user-message-block 
                [id]="item.id"
                [content]="item.content">
              </llm-user-message-block>
              <ng-content></ng-content>
            }
          </div>
        }
      }
    </div>
  `
})
export class BlockRouterComponent {
  public chatItem = input.required<ChatItem>();
  public facade = inject(TimelineFacade);

  public asFlowExecution(item: ChatItem): FlowExecutionChatItem {
    return item as FlowExecutionChatItem;
  }

  public asFileDelta(item: ChatItem): FileDeltaChatItem {
    return item as FileDeltaChatItem;
  }

  public asEnrichedModel(item: ChatItem): EnrichedChatItem {
    return item as EnrichedChatItem;
  }

  public asPrompt(item: ChatItem): MessageChatItem {
    return item as MessageChatItem;
  }
}