import { Component, EventEmitter, Input, Output } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'lib-selection-panel',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './selection-panel.component.html',
})
export class SelectionPanelComponent {
  @Input() files: Set<string> = new Set();
  @Input() missingFiles: Set<string> = new Set();
  @Input() currentFilename: string | null = null;
  @Input() hasUnsavedChanges = false;
  
  @Output() removeFile = new EventEmitter<string>();
  @Output() removeMissing = new EventEmitter<void>();
  @Output() quickSave = new EventEmitter<void>();
}