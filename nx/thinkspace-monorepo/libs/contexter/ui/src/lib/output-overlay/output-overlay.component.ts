import { Component, EventEmitter, Output, input, signal, computed } from '@angular/core';
import { CommonModule } from '@angular/common';

export interface SaveContextPayload {
  filename: string;
  description: string;
}

@Component({
  selector: 'lib-output-overlay',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './output-overlay.component.html'
})
export class OutputOverlayComponent {
  content = input<string>('');
  defaultFilename = input<string>('context_001.md');
  suggestedNewFilename = input<string>('');
  existingFiles = input<string[]>([]);

  @Output() copy = new EventEmitter<string>();
  @Output() save = new EventEmitter<SaveContextPayload>();
  @Output() close = new EventEmitter<void>();

  isSaveOpen = signal(false);
  customFilename = '';
  customDescription = '';
  private userHasEditedFilename = false;

  estimatedTokens = computed(() => Math.ceil(this.content().length / 4));
  
  estimatedFileSize = computed(() => {
    const bytes = this.content().length;
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  });

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

  toggleSavePanel() {
    this.isSaveOpen.update(v => !v);
    if (this.isSaveOpen()) {
      this.customFilename = this.defaultFilename();
      this.userHasEditedFilename = false;
      this.customDescription = '';
    }
  }

  updateFilename(event: Event) {
    this.customFilename = (event.target as HTMLInputElement).value;
    this.userHasEditedFilename = true;
  }

  updateDescription(event: Event) {
    this.customDescription = (event.target as HTMLInputElement).value;
  }

  onFilenameFocus() {
    if (!this.userHasEditedFilename && !this.existingFiles().includes(this.defaultFilename())) {
      this.customFilename = '';
    }
    this.userHasEditedFilename = true;
  }

  useIncrementedName() {
    this.customFilename = this.incrementedFilename();
    this.userHasEditedFilename = true;
  }

  willOverwrite(): boolean {
    const name = (this.userHasEditedFilename && this.customFilename.trim()) 
      ? this.customFilename.trim() 
      : this.defaultFilename();
    let finalName = name;
    if (finalName && !finalName.endsWith('.md')) finalName += '.md';
    return this.existingFiles().includes(finalName);
  }

  submitSave() {
    let filename = (this.userHasEditedFilename && this.customFilename.trim()) 
      ? this.customFilename.trim() 
      : this.defaultFilename();
      
    if (filename && !filename.endsWith('.md')) filename += '.md';
      
    this.save.emit({
      filename,
      description: this.customDescription.trim()
    });
    this.isSaveOpen.set(false);
  }

  onClose() {
    this.isSaveOpen.set(false);
    this.close.emit();
  }
}