import { Component, input, output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { MarkdownComponent } from '@org/llm-ui-core';
import { FileDeltaBlockComponent } from './file-delta-block/file-delta-block.component';
import { ChatItem, FileDeltaChatItem } from '@org/llm-state-chat';

@Component({
  selector: 'llm-model-message-block',
  standalone: true,
  imports: [CommonModule, MarkdownComponent, FileDeltaBlockComponent],
  template: `
    <div 
      class="rounded-xl border shadow-sm transition-all duration-300 overflow-hidden"
      [ngClass]="{
        'bg-gray-50 border-gray-200 p-4 hover:shadow-md': !isCollapsed(),
        'bg-gray-100 border-transparent p-1 h-2 opacity-40 cursor-pointer': isCollapsed(),
        'opacity-30 scale-[0.98]': isDimmed() && !isCollapsed(),
        'opacity-100 scale-100 ring-2 ring-blue-300': !isDimmed() && hasActiveTag()
      }"
    >
      @if (!isCollapsed()) {
        <div class="flex justify-between items-start mb-3">
          <strong class="text-[11px] font-bold uppercase tracking-wider text-gray-500">
            {{ source() }}
          </strong>
          
          <!-- Tag Group -->
          @if (tags().length > 0) {
            <div class="flex gap-1.5 flex-wrap justify-end">
              @for (tag of tags(); track tag) {
                <span 
                  class="px-2 py-0.5 text-[10px] font-mono font-semibold rounded-full border cursor-pointer transition-colors"
                  [ngClass]="activeTag() === tag ? 'bg-blue-100 text-blue-800 border-blue-300' : 'bg-white text-gray-500 border-gray-200 hover:bg-gray-100'"
                  role="button"
                  tabindex="0"
                  (mouseenter)="tagHover.emit(tag)"
                  (mouseleave)="tagHover.emit(null)"
                  (click)="tagClick.emit(tag)"
                  (keydown.enter)="tagClick.emit(tag)">
                  #{{ tag }}
                </span>
              }
            </div>
          }
        </div>
        
        <div class="text-gray-800 text-sm leading-relaxed">
          <llm-markdown [content]="content()"></llm-markdown>
        </div>

        <!-- Render Nested Workspace Attachments -->
        @if (attachments().length > 0) {
          <div class="mt-4 flex flex-col gap-2 border-t border-gray-200 pt-3">
            @for (att of attachments(); track att.id) {
              @if (att.type === 'file_delta') {
                @let fileItem = asFileDelta(att);
                <llm-file-delta-block
                  [path]="fileItem.path"
                  [summary]="fileItem.content"
                  [previewSnippet]="fileItem.previewSnippet"
                  [fileRef]="fileItem.fileRef">
                </llm-file-delta-block>
              }
            }
          </div>
        }
      }
    </div>
  `
})
export class ModelMessageBlockComponent {
  public source = input.required<string>();
  public content = input.required<string>();
  public tags = input<string[]>([]);
  public attachments = input<ChatItem[]>([]);
  
  public isDimmed = input<boolean>(false);
  public isCollapsed = input<boolean>(false);
  public activeTag = input<string | null>(null);

  public tagHover = output<string | null>();
  public tagClick = output<string>();

  public asFileDelta(item: ChatItem): FileDeltaChatItem {
    return item as FileDeltaChatItem;
  }

  public hasActiveTag(): boolean {
    const active = this.activeTag();
    return active !== null && this.tags().includes(active);
  }
}