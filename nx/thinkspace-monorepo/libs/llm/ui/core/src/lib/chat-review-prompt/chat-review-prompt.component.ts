import { Component, input, output, signal } from '@angular/core';

export interface ReviewDecisionEvent {
  accepted: boolean;
  reason: string;
}

@Component({
  selector: 'llm-chat-review-prompt',
  standalone: true,
  template: `
    <div
      style="background: #d1e7dd; padding: 16px; border: 1px solid #a3cfbb; border-radius: 4px; margin-top: 16px;"
    >
      <strong>👀 Previewing {{ branch() }}...</strong>
      <p style="margin: 8px 0; font-size: 0.9em; font-family: monospace;">
        📂 Files have been checked out locally. Open IDE to inspect.
      </p>
      <div style="margin-top: 10px;">
        <input
          type="text"
          [value]="reason()"
          (input)="updateReason($event)"
          (keydown.enter)="$event.preventDefault(); decide(true)"
          placeholder="Optional comment or reason (passed to next prompt turn)..."
          style="width: 100%; box-sizing: border-box; padding: 6px 10px; border: 1px solid #a3cfbb; border-radius: 4px; background: #fff; font-size: 13px;"
        />
      </div>
      <div style="display: flex; gap: 8px; margin-top: 12px;">
        <button
          (click)="decide(true)"
          style="padding: 6px 12px; cursor: pointer; background: #198754; color: white; border: none; border-radius: 4px; font-weight: bold;"
        >
          Accept (y)
        </button>
        <button
          (click)="decide(false)"
          style="padding: 6px 12px; cursor: pointer; background: #dc3545; color: white; border: none; border-radius: 4px; font-weight: bold;"
        >
          Reject (n)
        </button>
      </div>
    </div>
  `,
})
export class ChatReviewPromptComponent {
  branch = input.required<string>();
  reason = signal<string>('');
  reviewDecided = output<ReviewDecisionEvent>();

  updateReason(event: Event): void {
    const target = event.target as HTMLInputElement;
    this.reason.set(target.value);
  }

  decide(accepted: boolean) {
    this.reviewDecided.emit({
      accepted,
      reason: this.reason().trim(),
    });
  }
}