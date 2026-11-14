import { Component, output, signal } from '@angular/core';
import { DelegationStrategy } from '@org/llm-core-facade';

export interface StrategySelectionEvent {
  strategy: DelegationStrategy;
  autoAccept: boolean;
}

@Component({
  selector: 'llm-chat-strategy-prompt',
  standalone: true,
  template: `
    <div
      style="background: #fff3cd; padding: 16px; border: 1px solid #ffe69c; border-radius: 4px; margin-top: 16px;"
    >
      <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 8px;">
        <strong>Select Next Step:</strong>
        <label style="display: flex; align-items: center; gap: 6px; font-size: 12px; color: #664d03; cursor: pointer;">
          <input
            type="checkbox"
            [checked]="autoAccept()"
            (change)="toggleAutoAccept($event)"
          />
          Auto-accept synthesized result (skip final approval)
        </label>
      </div>
      <div style="display: flex; gap: 8px; margin-top: 12px; flex-wrap: wrap;">
        <button
          (click)="select(DelegationStrategy.MANUAL)"
          style="padding: 6px 12px; cursor: pointer;"
        >
          [1] Manual Review
        </button>
        <button
          (click)="select(DelegationStrategy.REVIEW)"
          style="padding: 6px 12px; cursor: pointer;"
        >
          [2] Assisted Review
        </button>
        <button
          (click)="select(DelegationStrategy.REFINE)"
          style="padding: 6px 12px; cursor: pointer;"
        >
          [3] Auto-Refine
        </button>
        <button
          (click)="select(DelegationStrategy.SKIP)"
          style="padding: 6px 12px; cursor: pointer;"
        >
          [0] Skip / Abort
        </button>
      </div>
    </div>
  `,
})
export class ChatStrategyPromptComponent {
  DelegationStrategy = DelegationStrategy;
  autoAccept = signal<boolean>(false);
  strategySelected = output<StrategySelectionEvent>();

  toggleAutoAccept(event: Event): void {
    const target = event.target as HTMLInputElement;
    this.autoAccept.set(target.checked);
  }

  select(strategy: DelegationStrategy) {
    this.strategySelected.emit({
      strategy,
      autoAccept: strategy === DelegationStrategy.REFINE ? this.autoAccept() : false,
    });
  }
}