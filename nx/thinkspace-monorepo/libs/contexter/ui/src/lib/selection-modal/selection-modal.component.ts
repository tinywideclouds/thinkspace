import { Component, EventEmitter, Output, input, effect, computed } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'lib-selection-modal',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './selection-modal.component.html'
})
export class SelectionModalComponent {
  mode = input<'save' | 'load' | null>(null);
  defaultFilename = input<string>('');
  suggestedNewFilename = input<string>('');
  existingFiles = input<string[]>([]);

  @Output() save = new EventEmitter<{filename: string}>();
  @Output() load = new EventEmitter<string>();
  @Output() close = new EventEmitter<void>();

  customFilename = '';
  private userEdited = false;

  constructor() {
    effect(() => {
      if (this.mode() === 'save' && !this.userEdited) {
        this.customFilename = this.defaultFilename();
      }
    });
  }

  incrementedFilename = computed(() => {
    const def = this.defaultFilename();
    if (!def) return '';
    const match = def.match(/^(.*?)_?(\d*)(\.[a-zA-Z0-9]+)$/);
    if (!match) return def;
    
    const base = match[1];
    const numStr = match[2];
    const ext = match[3];
    
    if (!numStr) return `${base}_01${ext}`;
    
    const num = parseInt(numStr, 10) + 1;
    const paddedNum = num.toString().padStart(numStr.length, '0');
    return `${base}_${paddedNum}${ext}`;
  });

  get canSaveAsNew(): boolean {
    return this.existingFiles().includes(this.defaultFilename()) && this.customFilename !== this.incrementedFilename();
  }

  updateFilename(e: Event) {
    this.customFilename = (e.target as HTMLInputElement).value;
    this.userEdited = true;
  }

  onFocus() {
    if (!this.userEdited && !this.existingFiles().includes(this.defaultFilename())) {
      this.customFilename = '';
    }
    this.userEdited = true;
  }

  useIncrementedName() {
    this.customFilename = this.incrementedFilename();
    this.userEdited = true;
  }

  willOverwrite(): boolean {
    const name = (this.userEdited && this.customFilename.trim()) ? this.customFilename.trim() : this.defaultFilename();
    let finalName = name;
    if (finalName && !finalName.endsWith('.yaml')) finalName += '.yaml';
    return this.existingFiles().includes(finalName);
  }

  submitSave() {
    let filename = (this.userEdited && this.customFilename.trim()) ? this.customFilename.trim() : this.defaultFilename();
    if (filename && !filename.endsWith('.yaml')) filename += '.yaml';
    this.save.emit({ filename });
    this.userEdited = false;
  }
}