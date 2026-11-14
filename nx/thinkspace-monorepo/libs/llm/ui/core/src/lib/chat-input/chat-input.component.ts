import { Component, output, viewChild, ElementRef } from '@angular/core';

@Component({
  selector: 'llm-chat-input',
  standalone: true,
  template: `
    <div class="flex items-end gap-2 mt-4">
      <textarea
        #promptInput
        rows="1"
        (input)="autoResize(promptInput)"
        (keydown)="onKeydown($event, promptInput)"
        placeholder="Type your message... (Shift+Enter for new line)"
        class="flex-1 px-3 py-2 text-sm leading-6 border border-gray-300 rounded-md resize-none overflow-y-hidden max-h-52 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
      ></textarea>
      <button
        type="button"
        (click)="submitAndReset(promptInput)"
        class="px-4 py-2 h-10 shrink-0 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-700 cursor-pointer transition-colors"
      >
        Send
      </button>
    </div>
  `,
})
export class ChatInputComponent {
  private static readonly MAX_HEIGHT_REM = 13; // 8 lines * 1.5rem line-height + 1rem vertical padding

  public promptInput = viewChild<ElementRef<HTMLTextAreaElement>>('promptInput');
  public sendPrompt = output<string>();

  public onKeydown(event: KeyboardEvent, textarea: HTMLTextAreaElement): void {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      this.submitAndReset(textarea);
    }
  }

  public submitAndReset(textarea: HTMLTextAreaElement): void {
    this.submit(textarea.value);
    textarea.value = '';
    this.resetHeight(textarea);
  }

  public submit(rawText: string): void {
    const text = rawText.trim();
    if (text) {
      this.sendPrompt.emit(text);
    }
  }

  public autoResize(textarea: HTMLTextAreaElement): void {
    const rootFontSize = parseFloat(getComputedStyle(document.documentElement).fontSize) || 16;
    const maxHeightPx = ChatInputComponent.MAX_HEIGHT_REM * rootFontSize;

    textarea.style.height = 'auto';
    const nextRem = Math.min(textarea.scrollHeight, maxHeightPx) / rootFontSize;
    textarea.style.height = `${nextRem}rem`;
    textarea.style.overflowY = textarea.scrollHeight > maxHeightPx ? 'auto' : 'hidden';
  }

  private resetHeight(textarea: HTMLTextAreaElement): void {
    textarea.style.height = 'auto';
    textarea.style.overflowY = 'hidden';
  }
}