import { Component, input, signal, computed } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'llm-file-delta-block',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="my-2 border border-gray-200 rounded-lg overflow-hidden bg-white shadow-sm transition-all duration-300">
      
      <!-- Collapsed / Header View -->
      <div 
        class="flex justify-between items-center px-4 py-3 bg-gray-50 cursor-pointer hover:bg-gray-100 transition-colors"
        (click)="isExpanded.set(!isExpanded())">
        <div class="flex items-center gap-3">
          <span class="text-lg">{{ isExpanded() ? '📂' : '📁' }}</span>
          <div class="flex flex-col">
            <span class="font-mono text-xs font-bold text-blue-700">{{ path() }}</span>
            <span class="text-xs text-gray-500">{{ summary() }}</span>
          </div>
        </div>
        <div class="flex gap-2 items-center">
          <button class="px-3 py-1 bg-white border border-gray-300 rounded text-xs font-bold text-gray-600 hover:text-blue-600 hover:border-blue-300 transition-colors">
            {{ isExpanded() ? 'Collapse' : 'Expand' }}
          </button>
        </div>
      </div>

      <!-- Expanded Zoom View -->
      @if (isExpanded()) {
        <div class="border-t border-gray-200 bg-gray-900 text-gray-100 font-mono text-[11px] overflow-x-auto max-h-[60vh] overflow-y-auto">
          <table class="w-full border-collapse">
            <tbody>
              @for (line of parsedLines(); track $index) {
                <tr [class.bg-green-900]="isLineHighlighted($index)" 
                    [class.bg-opacity-40]="isLineHighlighted($index)">
                  <td class="px-2 py-0.5 text-gray-500 select-none text-right border-r border-gray-700 w-12">
                    {{ $index + 1 }}
                  </td>
                  <td class="px-4 py-0.5 whitespace-pre"
                      [class.text-green-400]="isLineHighlighted($index)">
                    {{ line || ' ' }}
                  </td>
                </tr>
              }
            </tbody>
          </table>
        </div>
      }
    </div>
  `
})
export class FileDeltaBlockComponent {
  path = input.required<string>();
  summary = input.required<string>();
  fullContent = input.required<string>();
  highlightLines = input<number[]>([]);

  isExpanded = signal<boolean>(false);

  parsedLines = computed(() => {
    const content = this.fullContent();
    return content ? content.split('\n') : [];
  });

  isLineHighlighted(index: number): boolean {
    // Convert 0-based array index to 1-based line number
    return this.highlightLines().includes(index + 1);
  }
}