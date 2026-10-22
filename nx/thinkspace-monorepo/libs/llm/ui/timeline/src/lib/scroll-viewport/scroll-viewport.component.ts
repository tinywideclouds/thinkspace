import { 
  Component, 
  ElementRef, 
  viewChild, 
  afterNextRender, 
  inject, 
  DestroyRef, 
  output, 
  signal 
} from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'llm-scroll-viewport',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './scroll-viewport.component.html'
})
export class ScrollViewportComponent {
  public containerRef = viewChild.required<ElementRef<HTMLDivElement>>('scrollContainer');
  public topSentinelRef = viewChild.required<ElementRef<HTMLDivElement>>('topSentinel');
  public bottomSentinelRef = viewChild.required<ElementRef<HTMLDivElement>>('bottomSentinel');

  public loadPreviousPage = output<void>();

  public isAtBottom = signal<boolean>(true);
  public unreadIndicatorActive = signal<boolean>(false);

  private destroyRef = inject(DestroyRef);

  constructor() {
    afterNextRender(() => {
      this.scrollToBottom();
      this.setupObservers();
    });
  }

  public scrollToBottom(): void {
    const container = this.containerRef().nativeElement;
    container.scrollTop = container.scrollHeight;
    this.unreadIndicatorActive.set(false);
  }

  private setupObservers(): void {
    const container = this.containerRef().nativeElement;

    const topObserver = new IntersectionObserver(([entry]) => {
      if (entry.isIntersecting) {
        this.loadPreviousPage.emit();
      }
    }, { root: container, threshold: 0.1 });
    topObserver.observe(this.topSentinelRef().nativeElement);

    const bottomObserver = new IntersectionObserver(([entry]) => {
      this.isAtBottom.set(entry.isIntersecting);
      if (entry.isIntersecting) {
        this.unreadIndicatorActive.set(false); 
      }
    }, { root: container, threshold: 0.1 });
    bottomObserver.observe(this.bottomSentinelRef().nativeElement);

    const mutationObserver = new MutationObserver((mutations) => {
      const domChanged = mutations.some(m => m.addedNodes.length > 0 || m.type === 'characterData');
      if (domChanged) {
        if (this.isAtBottom()) {
          container.scrollTop = container.scrollHeight;
        } else {
          this.unreadIndicatorActive.set(true);
        }
      }
    });

    mutationObserver.observe(container.firstElementChild as Node, {
      childList: true,
      subtree: true,
      characterData: true
    });

    this.destroyRef.onDestroy(() => {
      topObserver.disconnect();
      bottomObserver.disconnect();
      mutationObserver.disconnect();
    });
  }
}