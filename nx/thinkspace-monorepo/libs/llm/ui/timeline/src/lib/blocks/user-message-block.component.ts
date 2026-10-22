import { Component, input } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'llm-user-message-block',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="p-4 rounded-xl bg-blue-50 border border-blue-200 ml-8 shadow-sm">
      <strong class="block mb-2 text-[11px] font-bold uppercase tracking-wider text-blue-700">
        User
      </strong>
      <div class="whitespace-pre-wrap text-sm text-blue-900">
        {{ content() }}
      </div>
    </div>
  `
})
export class UserMessageBlockComponent {
  public content = input.required<string>();
}