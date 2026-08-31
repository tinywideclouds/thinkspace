import { Component, EventEmitter, Input, Output } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'lib-output-overlay',
  standalone: true,
  imports: [CommonModule],
  template: `
    @if (content) {
      <div class="absolute inset-0 z-50 bg-black/50 flex items-center justify-center p-8 backdrop-blur-sm">
        <div class="bg-white rounded-lg shadow-2xl w-full max-w-5xl h-full max-h-[80vh] flex flex-col">
          <div class="p-4 border-b border-gray-200 flex justify-between items-center bg-gray-50 rounded-t-lg shrink-0">
            <h3 class="font-bold text-lg">LLM Context</h3>
            <div class="flex gap-4">
              <button 
                (click)="copy.emit(content)" 
                class="text-sm font-medium text-blue-600 hover:underline">
                Copy to Clipboard
              </button>
              <button 
                (click)="save.emit()" 
                class="text-sm font-medium text-blue-600 hover:underline">
                Save to Disk
              </button>
              <button 
                (click)="close.emit()" 
                class="text-sm font-bold text-gray-500 hover:text-gray-800">
                Close
              </button>
            </div>
          </div>
          <div class="flex-1 overflow-auto p-4 bg-gray-900 rounded-b-lg">
            <pre class="text-gray-100 text-sm whitespace-pre-wrap font-mono">{{ content }}</pre>
          </div>
        </div>
      </div>
    }
  `
})
export class OutputOverlayComponent {
  @Input() content = '';
  @Output() copy = new EventEmitter<string>();
  @Output() save = new EventEmitter<void>();
  @Output() close = new EventEmitter<void>();
}