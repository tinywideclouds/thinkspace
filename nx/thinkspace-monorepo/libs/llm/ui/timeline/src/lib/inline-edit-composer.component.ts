import { Component, input, output, signal, effect, viewChild, ElementRef, afterNextRender } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'llm-inline-edit-composer',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="p-4 rounded-xl bg-white border-2 border-blue-400 shadow-md ml-8 relative z-20">
      <div class="flex justify-between items-center mb-2">
        <strong class="text-[11px] font-bold uppercase tracking-wider text-blue-700">
          Amend Prompt
        </strong>
        <span class="text-[10px] text-gray-400 font-mono">Will fork branch</span>
      </div>
      
      <textarea
        #composerInput
        class="w-full min-h-[100px] p-3 text-sm text-gray-800 bg-gray-50 border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent resize-y"
        [value]="editText()"
        (input)="updateText($event)"
        (keydown.escape)="exit.emit()"
        (keydown.control.enter)="onSave()">
      </textarea>
      
      <div class="flex justify-end gap-2 mt-3">
        <button 
          (click)="exit.emit()"
          class="px-4 py-1.5 text-xs font-bold text-gray-500 hover:text-gray-800 hover:bg-gray-100 rounded transition-colors">
          Cancel (Esc)
        </button>
        <button 
          (click)="onSave()"
          [disabled]="!editText().trim() || editText() === initialText()"
          class="px-4 py-1.5 text-xs font-bold text-white bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed rounded shadow-sm transition-all">
          Save & Amend (Ctrl+Enter)
        </button>
      </div>
    </div>
  `
})
export class InlineEditComposerComponent {
  public initialText = input.required<string>();
  
  public save = output<string>();
  public exit = output<void>();

  public editText = signal<string>('');
  public inputRef = viewChild<ElementRef<HTMLTextAreaElement>>('composerInput');

  constructor() {
    effect(() => {
      this.editText.set(this.initialText());
    }, { allowSignalWrites: true });

    afterNextRender(() => {
      const el = this.inputRef()?.nativeElement;
      if (el) {
        el.focus();
        el.setSelectionRange(el.value.length, el.value.length);
      }
    });
  }

  public updateText(event: Event): void {
    const target = event.target as HTMLTextAreaElement;
    this.editText.set(target.value);
  }

  public onSave(): void {
    const text = this.editText().trim();
    if (text && text !== this.initialText()) {
      this.save.emit(text);
    }
  }
}