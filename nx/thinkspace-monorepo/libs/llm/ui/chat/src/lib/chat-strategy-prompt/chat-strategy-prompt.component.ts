import { Component, output } from '@angular/core';
import { DomainDelegationStrategy } from '@org/llm-core-facade';

@Component({
  selector: 'llm-chat-strategy-prompt',
  standalone: true,
  template: `
    <div style="background: #fff3cd; padding: 16px; border: 1px solid #ffe69c; border-radius: 4px; margin-top: 16px;">
      <strong>Select Next Step:</strong>
      <div style="display: flex; gap: 8px; margin-top: 12px; flex-wrap: wrap;">
        <button (click)="select(DomainDelegationStrategy.MANUAL)" style="padding: 6px 12px; cursor: pointer;">[1] Manual Review</button>
        <button (click)="select(DomainDelegationStrategy.REVIEW)" style="padding: 6px 12px; cursor: pointer;">[2] Assisted Review</button>
        <button (click)="select(DomainDelegationStrategy.REFINE)" style="padding: 6px 12px; cursor: pointer;">[3] Auto-Refine</button>
        <button (click)="select(DomainDelegationStrategy.SKIP)" style="padding: 6px 12px; cursor: pointer;">[0] Skip / Abort</button>
      </div>
    </div>
  `
})
export class ChatStrategyPromptComponent {
  DomainDelegationStrategy = DomainDelegationStrategy;
  strategySelected = output<DomainDelegationStrategy>();

  select(strategy: DomainDelegationStrategy) {
    this.strategySelected.emit(strategy);
  }
}