import { Component, input, output } from '@angular/core';

@Component({
  selector: 'llm-chat-transition-prompt',
  standalone: true,
  template: `
    <div style="background: #fff3cd; padding: 16px; border: 1px solid #ffe69c; border-radius: 4px; margin-top: 16px;">
      <strong>⚠️ Workspace Transition Requested</strong>
      <p style="margin: 8px 0; font-size: 0.9em; color: #495057;">
        The Manager wants to transition the workspace to the <strong style="color: #212529;">{{ targetDomain() }}</strong> domain.
      </p>
      <p style="margin: 8px 0; font-size: 0.9em; font-style: italic; color: #495057;">
        Reasoning: {{ reasoning() }}
      </p>
      <div style="display: flex; gap: 8px; margin-top: 12px;">
        <button (click)="decide(true)" style="padding: 6px 12px; cursor: pointer; background: #198754; color: white; border: none; border-radius: 4px; font-weight: bold;">Approve & Scaffold</button>
        <button (click)="decide(false)" style="padding: 6px 12px; cursor: pointer; background: #dc3545; color: white; border: none; border-radius: 4px; font-weight: bold;">Reject</button>
      </div>
    </div>
  `
})
export class ChatTransitionPromptComponent {
  targetDomain = input.required<string>();
  reasoning = input.required<string>();
  transitionDecided = output<boolean>();

  decide(approved: boolean) {
    this.transitionDecided.emit(approved);
  }
}