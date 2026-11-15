import { Component, input, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TimelineFacade } from '../timeline.facade';
import { ChatConnectionService } from '@org/llm-state-chat';
import { InlineEditComposerComponent } from '../inline-edit-composer.component';

@Component({
  selector: 'llm-user-message-block',
  standalone: true,
  imports: [CommonModule, InlineEditComposerComponent],
  host: { class: 'block w-full' },
  template: `
    @if (isEditing()) {
      <llm-inline-edit-composer
        [initialText]="content()"
        (save)="onSave($event)"
        (cancel)="isEditing.set(false)">
      </llm-inline-edit-composer>
    } @else {
      <div class="p-4 rounded-xl bg-blue-50 border border-blue-200 ml-8 shadow-sm relative group transition-all duration-200">
        <div class="flex justify-between items-start mb-2">
          <strong class="block text-[11px] font-bold uppercase tracking-wider text-blue-700">
            User
          </strong>
          
          @if (facade.isEditMode()) {
            <div class="flex gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
              <button 
                (click)="isEditing.set(true)"
                class="px-2 py-1 bg-white border border-blue-200 rounded text-[10px] font-bold text-blue-600 hover:bg-blue-100 transition-colors shadow-sm">
                ✏️ Edit
              </button>
              <button 
                (click)="onFork()"
                class="px-2 py-1 bg-white border border-blue-200 rounded text-[10px] font-bold text-blue-600 hover:bg-blue-100 transition-colors shadow-sm">
                🌿 Fork
              </button>
            </div>
          }
        </div>
        
        <div class="whitespace-pre-wrap text-sm text-blue-900">
          {{ content() }}
        </div>

        @if (metadata()['transitioned_to']) {
          <div class="mt-3 flex items-center">
            <span class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md bg-blue-100 text-blue-800 text-[10px] font-bold uppercase tracking-wider border border-blue-200 shadow-sm select-none">
              ⚡ Transitioned to: {{ metadata()['transitioned_to'] }}
            </span>
          </div>
        }
      </div>
    }
  `
})
export class UserMessageBlockComponent {
  public id = input.required<string>();
  public content = input.required<string>();
  public metadata = input<Record<string, string>>({});
  
  public facade = inject(TimelineFacade);
  public connection = inject(ChatConnectionService);
  
  public isEditing = signal<boolean>(false);

  public onSave(newText: string): void {
    this.connection.inlineEdit(this.id(), newText);
    this.isEditing.set(false);
    this.facade.toggleEditMode();
  }

  public onFork(): void {
    this.connection.forkChat(this.id());
    this.facade.toggleEditMode();
  }
}