import { Component, input, signal, computed, inject, viewChild, ElementRef } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TimelineFacade } from '../../timeline.facade';
import { ScrollViewportComponent } from '../../scroll-viewport/scroll-viewport.component';

@Component({
  selector: 'llm-file-delta-block',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './file-delta-block.component.html',
  styleUrl: './file-delta-block.component.scss'
})
export class FileDeltaBlockComponent {
  public path = input.required<string>();
  public summary = input.required<string>();
  public previewSnippet = input.required<string>();
  public fileRef = input.required<string>();
  public highlightLines = input<number[]>([]);

  public containerRef = viewChild.required<ElementRef<HTMLElement>>('blockContainer');
  public facade = inject(TimelineFacade);
  private viewport = inject(ScrollViewportComponent, { optional: true });

  public isExpanded = signal<boolean>(false);
  public isLoading = signal<boolean>(false);
  public fullContent = signal<string | null>(null);

  public displayLines = computed(() => {
    const content = this.fullContent() || this.previewSnippet();
    return content ? content.split('\n') : [];
  });

  public async toggleExpand(): Promise<void> {
    const willExpand = !this.isExpanded();
    
    if (willExpand) {
      // User interaction breaks the tail lock. 
      // Native overflow-anchor will keep this block pinned in place as it expands.
      this.viewport?.scrollState.set('manual');
    }
    
    this.isExpanded.set(willExpand);

    if (willExpand && !this.fullContent()) {
      this.isLoading.set(true);
      try {
        const content = await this.facade.getFileContent(this.path(), this.fileRef());
        this.fullContent.set(content);
      } catch (error) {
        console.error('[FileDeltaBlockComponent] Failed to fetch full file content:', error);
        this.fullContent.set('// Error: Could not load full file content from workspace.\n' + this.previewSnippet());
      } finally {
        this.isLoading.set(false);
      }
    }
  }

  public isLineHighlighted(index: number): boolean {
    return this.highlightLines().includes(index + 1);
  }
}