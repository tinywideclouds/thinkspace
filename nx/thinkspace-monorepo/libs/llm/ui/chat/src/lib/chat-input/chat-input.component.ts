import { Component, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'llm-chat-input',
  standalone: true,
  imports: [FormsModule],
  template: `
    <div style="display: flex; gap: 8px; margin-top: 16px;">
      <input 
        type="text" 
        [(ngModel)]="promptText" 
        (keyup.enter)="submit()"
        placeholder="Type your message..." 
        style="flex: 1; padding: 8px; border: 1px solid #ccc; border-radius: 4px;"
      />
      <button (click)="submit()" style="padding: 8px 16px; cursor: pointer;">Send</button>
    </div>
  `
})
export class ChatInputComponent {
  promptText = signal('');
  sendPrompt = output<string>();

  submit() {
    const text = this.promptText().trim();
    if (text) {
      this.sendPrompt.emit(text);
      this.promptText.set('');
    }
  }
}