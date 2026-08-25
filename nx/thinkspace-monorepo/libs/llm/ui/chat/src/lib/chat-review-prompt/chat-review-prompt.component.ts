import { Component, input, output } from '@angular/core';

@Component({
  selector: 'llm-chat-review-prompt',
  standalone: true,
  template: `
    <div style="background: #d1e7dd; padding: 16px; border: 1px solid #a3cfbb; border-radius: 4px; margin-top: 16px;">
      <strong>👀 Previewing {{ branch() }}...</strong>
      <p style="margin: 8px 0; font-size: 0.9em; font-family: monospace;">📂 Files have been checked out locally. Open IDE to inspect.</p>
      <div style="display: flex; gap: 8px; margin-top: 12px;">
        <button (click)="decide(true)" style="padding: 6px 12px; cursor: pointer; background: #198754; color: white; border: none; border-radius: 4px;">Accept (y)</button>
        <button (click)="decide(false)" style="padding: 6px 12px; cursor: pointer; background: #dc3545; color: white; border: none; border-radius: 4px;">Reject (n)</button>
      </div>
    </div>
  `
})
export class ChatReviewPromptComponent {
  branch = input.required<string>();
  reviewDecided = output<boolean>();

  decide(accepted: boolean) {
    this.reviewDecided.emit(accepted);
  }
}