import { Component, input, output } from '@angular/core';
import { CommonModule } from '@angular/common';

export interface HeaderSpace {
  id: string;
  name: string;
  isConfigured: boolean;
}

export interface HeaderChat {
  id: string;
  name: string;
  createdAt: string;
}

@Component({
  selector: 'llm-chat-header',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div style="display: flex; gap: 24px; align-items: center; width: 100%;">
      <div style="display: flex; flex-direction: column; gap: 4px; flex: 1;">
        <label for="spaceSelect" style="font-size: 11px; color: #868e96; font-weight: bold; text-transform: uppercase; letter-spacing: 0.5px;">ThinkSpace</label>
        <select 
          id="spaceSelect" 
          [value]="activeSpaceId() || ''" 
          (change)="handleSpaceChange($event)" 
          style="padding: 8px; border: 1px solid #ced4da; border-radius: 4px; background: white; font-size: 14px; width: 100%;">
          <option value="" disabled>Select Space...</option>
          @for (space of spaces(); track space.id) {
            <option [value]="space.id">{{ space.name }} {{ space.isConfigured ? '' : '(Unconfigured)' }}</option>
          }
        </select>
      </div>

      <div style="display: flex; flex-direction: column; gap: 4px; flex: 2;">
        <label for="chatSelect" style="font-size: 11px; color: #868e96; font-weight: bold; text-transform: uppercase; letter-spacing: 0.5px;">Conversation Thread</label>
        <div style="display: flex; gap: 8px;">
          <select 
            id="chatSelect" 
            [value]="activeChatId() || ''" 
            (change)="handleChatChange($event)" 
            style="padding: 8px; border: 1px solid #ced4da; border-radius: 4px; background: white; font-size: 14px; width: 100%;">
            <option value="" disabled>Select Chat...</option>
            @for (chat of chats(); track chat.id) {
              <option [value]="chat.id">{{ chat.name }}</option>
            }
          </select>
          <button 
            (click)="newChatRequested.emit()" 
            style="padding: 8px 16px; background: #f8f9fa; border: 1px solid #ced4da; border-radius: 4px; cursor: pointer; font-weight: bold; color: #495057; white-space: nowrap;">
            + New
          </button>
        </div>
      </div>
    </div>
  `
})
export class ChatHeaderComponent {
  public spaces = input.required<HeaderSpace[]>();
  public activeSpaceId = input.required<string | null>();
  
  public chats = input.required<HeaderChat[]>();
  public activeChatId = input.required<string | null>();

  public spaceSelected = output<string>();
  public chatSelected = output<string>();
  public newChatRequested = output<void>();

  public handleSpaceChange(event: Event): void {
    const targetElement = event.target as HTMLSelectElement;
    if (targetElement.value) {
      this.spaceSelected.emit(targetElement.value);
    }
  }

  public handleChatChange(event: Event): void {
    const targetElement = event.target as HTMLSelectElement;
    if (targetElement.value) {
      this.chatSelected.emit(targetElement.value);
    }
  }
}