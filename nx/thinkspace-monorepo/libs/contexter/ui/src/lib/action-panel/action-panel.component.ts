import { Component, EventEmitter, Input, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ContextConfig } from '@org/contexter-shared';

@Component({
  selector: 'lib-action-panel',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './action-panel.component.html'
})
export class ActionPanelComponent {
  @Input() config: ContextConfig | null = null;
  @Input() selectedCount = 0;
  @Input() allowLargeFiles = false;

  @Output() allowLargeFilesChange = new EventEmitter<boolean>();
  @Output() generateContext = new EventEmitter<void>();
  @Output() saveSelection = new EventEmitter<void>();
  @Output() loadSelection = new EventEmitter<string>();
  @Output() clearSelection = new EventEmitter<void>();
}