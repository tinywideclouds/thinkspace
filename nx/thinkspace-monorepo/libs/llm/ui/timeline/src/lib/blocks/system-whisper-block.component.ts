import { Component, input } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'llm-system-whisper-block',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="flex items-center justify-center w-full my-1 opacity-80 select-none">
      <div class="h-px bg-gray-300 w-4 rounded-full"></div>
      <span class="text-[9px] font-mono text-gray-500 mx-2">⚡ {{ content() }}</span>
      <div class="h-px bg-gray-300 w-4 rounded-full"></div>
    </div>
  `
})
export class SystemWhisperBlockComponent {
  public content = input.required<string>();
}