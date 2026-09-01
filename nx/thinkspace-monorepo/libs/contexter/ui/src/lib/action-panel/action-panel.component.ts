import { Component, EventEmitter, Input, Output } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'lib-action-panel',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './action-panel.component.html'
})
export class ActionPanelComponent {
  @Input() config: any;
  @Input() selectedCount = 0;

  @Output() generateContext = new EventEmitter<void>();
  @Output() saveSelection = new EventEmitter<void>();
  @Output() loadSelection = new EventEmitter<string>();
  @Output() clearSelection = new EventEmitter<void>();
}