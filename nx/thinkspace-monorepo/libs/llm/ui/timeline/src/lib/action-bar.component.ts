import { Component, input, output } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'llm-action-bar',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div 
      class="absolute left-1/2 -translate-x-1/2 z-50 flex items-center gap-4 bg-gray-900 text-white px-6 py-3 rounded-full shadow-2xl transition-all duration-300 border border-gray-700"
      [ngClass]="position() === 'top' ? 'top-6' : 'bottom-6'">
      
      <div class="flex items-center gap-2 pr-4 border-r border-gray-600">
        <span class="flex items-center justify-center bg-blue-600 w-6 h-6 rounded-full text-xs font-bold">
          {{ selectedCount() }}
        </span>
        <span class="text-sm font-bold tracking-wider uppercase text-gray-300">Turns Selected</span>
      </div>

      <div class="flex gap-2">
        <button 
          (click)="actionRequested.emit('tag')"
          class="px-3 py-1.5 bg-gray-800 hover:bg-gray-700 rounded text-xs font-bold transition-colors">
          🏷️ Tag Context
        </button>
        <button 
          (click)="actionRequested.emit('tombstone')"
          class="px-3 py-1.5 bg-gray-800 hover:bg-red-900/50 hover:text-red-300 rounded text-xs font-bold transition-colors">
          🪦 Tombstone
        </button>
        <button 
          (click)="actionRequested.emit('fork')"
          class="px-3 py-1.5 bg-gray-800 hover:bg-blue-900/50 hover:text-blue-300 rounded text-xs font-bold transition-colors">
          🌿 Fork Branch
        </button>
      </div>

      <button 
        (click)="cancelRequested.emit()"
        class="ml-2 pl-4 border-l border-gray-600 text-gray-400 hover:text-white text-xs font-bold tracking-widest uppercase transition-colors">
        Cancel
      </button>
    </div>
  `
})
export class ActionBarComponent {
  public selectedCount = input.required<number>();
  public position = input<'top' | 'bottom'>('top');

  public actionRequested = output<'tag' | 'tombstone' | 'fork'>();
  public cancelRequested = output<void>();
}