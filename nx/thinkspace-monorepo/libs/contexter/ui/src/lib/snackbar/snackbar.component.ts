import { Component, Input } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'lib-snackbar',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div 
      class="fixed bottom-6 left-1/2 transform -translate-x-1/2 transition-all duration-300 z-[100] flex items-center gap-2 px-4 py-3 bg-gray-900 text-white text-sm font-medium rounded-lg shadow-xl"
      [class.opacity-0]="!isVisible"
      [class.translate-y-12]="!isVisible"
      [class.opacity-100]="isVisible"
      [class.translate-y-0]="isVisible"
      [style.pointer-events]="isVisible ? 'auto' : 'none'">
      @if (type === 'success') {
        <span class="text-green-400">✓</span>
      } @else {
        <span class="text-red-400 font-bold">⚠️</span>
      }
      {{ message }}
    </div>
  `
})
export class SnackbarComponent {
  @Input() message = '';
  @Input() isVisible = false;
  @Input() type: 'success' | 'error' = 'success';
}