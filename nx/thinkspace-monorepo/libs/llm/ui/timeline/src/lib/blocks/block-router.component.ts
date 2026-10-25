import { Component, input, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ChatItem } from '@org/llm-state-chat';
import { TimelineFacade } from '../timeline.facade';
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
        
        @case ('flow_card') {
          <div [class.opacity-30]="facade.isItemDimmed(chatItem())" [class.h-2]="facade.isItemCollapsed(chatItem())" class="transition-all duration-300 overflow-hidden">
            @if (!facade.isItemCollapsed(chatItem())) {
              <llm-chat-flow-card 
                [flow]="facade.flowStates().get($any(chatItem()).flowId)" 
                [summary]="chatItem().content"
                (inspect)="facade.inspectFlow($event)">
              </llm-chat-flow-card>
            }
          </div>
        }

        @case ('model') {
          <llm-model-message-block 
            [source]="chatItem().type" 
            [content]="chatItem().content"
            [tags]="chatItem().tags || []"
            [attachments]="$any(chatItem()).attachments || []"
            [isDimmed]="facade.isItemDimmed(chatItem())"
            [isCollapsed]="facade.isItemCollapsed(chatItem())"
            [activeTag]="facade.hoveredTag() || facade.selectedTag()"
            (tagHover)="facade.setHoveredTag($event)"
            (tagClick)="facade.toggleSelectedTag($event)">
          </llm-model-message-block>
        }

        @case ('system') {
          <llm-model-message-block 
            [source]="chatItem().type" 
            [content]="chatItem().content"
            [tags]="chatItem().tags || []"
            [attachments]="$any(chatItem()).attachments || []"
            [isDimmed]="facade.isItemDimmed(chatItem())"
            [isCollapsed]="facade.isItemCollapsed(chatItem())"
            [activeTag]="facade.hoveredTag() || facade.selectedTag()"
            (tagHover)="facade.setHoveredTag($event)"
            (tagClick)="facade.toggleSelectedTag($event)">
          </llm-model-message-block>
        }

        @case ('file_delta') {
          <div [class.opacity-30]="facade.isItemDimmed(chatItem())" [class.h-2]="facade.isItemCollapsed(chatItem())" class="transition-all duration-300 overflow-hidden">
            @if (!facade.isItemCollapsed(chatItem())) {
              <llm-file-delta-block
                [path]="$any(chatItem()).path"
                [summary]="chatItem().content"
                [previewSnippet]="$any(chatItem()).previewSnippet"
                [fileRef]="$any(chatItem()).fileRef">
              </llm-file-delta-block>
            }
          </div>
        }

        @case ('user') {
          <div class="relative transition-all duration-300"
            [ngClass]="{
              'opacity-30 scale-[0.98]': facade.isItemDimmed(chatItem()) && !facade.isItemCollapsed(chatItem()),
              'h-2 opacity-40 py-0 mb-2 bg-blue-100 border-transparent overflow-hidden': facade.isItemCollapsed(chatItem())
            }">
            @if (!facade.isItemCollapsed(chatItem())) {
              <llm-user-message-block [content]="chatItem().content"></llm-user-message-block>
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
}