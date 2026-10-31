import { Component, input, computed } from '@angular/core';
import { CommonModule } from '@angular/common';

const TAILWIND_COLORS = [
  'bg-red-500', 'bg-orange-500', 'bg-amber-500', 'bg-green-500', 
  'bg-emerald-500', 'bg-teal-500', 'bg-cyan-500', 'bg-blue-500', 
  'bg-indigo-500', 'bg-violet-500', 'bg-purple-500', 'bg-fuchsia-500', 'bg-rose-500'
];

@Component({
  selector: 'llm-tag-ribbon',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="flex h-full w-2 gap-px py-2 opacity-80 transition-all hover:opacity-100">
      @for (color of ribbonColors(); track $index) {
        <div class="h-full flex-1 rounded-full shadow-sm" [ngClass]="color"></div>
      }
    </div>
  `
})
export class TagRibbonComponent {
  public tags = input.required<string[]>();

  public ribbonColors = computed(() => {
    return this.tags().map(tag => {
      let hash = 0;
      for (let i = 0; i < tag.length; i++) {
        hash = tag.charCodeAt(i) + ((hash << 5) - hash);
      }
      return TAILWIND_COLORS[Math.abs(hash) % TAILWIND_COLORS.length];
    });
  });
}