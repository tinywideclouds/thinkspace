import { Component, input, output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { DomainAttentionFocus } from '@org/llm-core-facade';

@Component({
  selector: 'llm-focus-hud',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './focus-hud.component.html',
  styleUrl: './focus-hud.component.scss'
})
export class FocusHudComponent {
  focus = input<DomainAttentionFocus | null>(null);
  activeHoverTag = input<string | null>(null);

  tagHovered = output<string | null>();
  tagClicked = output<string>();
}