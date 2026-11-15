import { Component, input, output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TagRibbonComponent } from '../tag-ribbon.component';

@Component({
  selector: 'llm-turn-group',
  standalone: true,
  imports: [CommonModule, TagRibbonComponent],
  host: { class: 'block w-full' },
  template: `
    <div 
      class="relative flex w-full transition-all duration-300 rounded-xl"
      [ngClass]="{
        'ring-2 ring-blue-400 bg-blue-50/30 shadow-sm my-2 p-2': isEditMode(),
        'opacity-50 grayscale': isEditMode() && !isSelected()
      }">
      
      <!-- Selection Checkbox (Edit Mode Only) -->
      @if (isEditMode()) {
        <div class="absolute -left-6 top-6 z-20">
          <input 
            type="checkbox" 
            class="w-4 h-4 cursor-pointer accent-blue-600"
            [checked]="isSelected()"
            (change)="selectionToggled.emit()">
        </div>
      }

      <!-- Tag Ribbon Spine -->
      @if (tags().length > 0) {
        <div class="absolute left-0 top-0 bottom-0 -ml-3 z-10">
          <llm-tag-ribbon [tags]="tags()"></llm-tag-ribbon>
        </div>
      }

      <!-- Core Blocks Container -->
      <div class="flex-1 flex flex-col w-full relative z-10 pl-2">
        <ng-content></ng-content>
      </div>
    </div>
  `
})
export class TurnGroupComponent {
  public isEditMode = input.required<boolean>();
  public isSelected = input.required<boolean>();
  public tags = input<string[]>([]);
  
  public selectionToggled = output<void>();
}