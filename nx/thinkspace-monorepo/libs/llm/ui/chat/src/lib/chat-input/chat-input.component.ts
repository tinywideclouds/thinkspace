import { Component, output } from '@angular/core';

@Component({
  selector: 'llm-chat-input',
  standalone: true,
  template: `
    <div style="display: flex; gap: 8px; margin-top: 16px;">
      <input 
        #promptInput
        type="text" 
        (keyup.enter)="submit(promptInput.value); promptInput.value=''"
        placeholder="Type your message..." 
        style="flex: 1; padding: 8px; border: 1px solid #ccc; border-radius: 4px;"
      />
      <button 
        (click)="submit(promptInput.value); promptInput.value=''" 
        style="padding: 8px 16px; cursor: pointer;">
        Send
      </button>
    </div>
  `
})
export class ChatInputComponent {
  sendPrompt = output<string>();

  submit(rawText: string) {
    const text = rawText.trim();
    if (text) {
      this.sendPrompt.emit(text);
    }
  }
}