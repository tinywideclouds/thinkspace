import { Component, input, output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FileContextMetaData } from '@org/llm-core-facade';

@Component({
  selector: 'llm-desk',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './desk.component.html',
  styleUrl: './desk.component.scss'
})
export class DeskComponent {
  items = input<FileContextMetaData[]>([]);
  fileSelected = output<string>();
}