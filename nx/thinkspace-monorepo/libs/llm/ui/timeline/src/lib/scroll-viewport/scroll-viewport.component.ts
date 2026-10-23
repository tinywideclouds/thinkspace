import { 
  Component, 
  ElementRef, 
  viewChild, 
  afterNextRender, 
  inject, 
  DestroyRef, 
  output, 
  signal,
  input,
  effect
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

  public ceilingAnchor = input<Element | null>(null);
  public topContentAnchor = input<Element | null>(null);
  public isDebugMode = input<boolean>(false);
  
  public loadPreviousPage = output<void>();

  public isAtBottom = signal<boolean>(true);
  public unreadIndicatorActive = signal<boolean>(false);
  
  public topContentVisible = signal<boolean>(false);
  public ceilingAnchorVisible = signal<boolean>(false);
  public ceilingLocked = signal<boolean>(false);

  private destroyRef = inject(DestroyRef);
  private dynamicObserver: IntersectionObserver | null = null;

  constructor() {
    effect(() => {
      const topAnchor = this.topContentAnchor();
      const ceiling = this.ceilingAnchor();
      const container = this.containerRef()?.nativeElement;

      if (!container) return;

      if (!this.dynamicObserver) {
        this.dynamicObserver = new IntersectionObserver((entries) => {
          entries.forEach(entry => {
            if (entry.target === this.topContentAnchor()) {
              this.topContentVisible.set(entry.isIntersecting);
            }
            if (entry.target === this.ceilingAnchor()) {
              this.ceilingAnchorVisible.set(entry.isIntersecting);
              
              const rootTop = entry.rootBounds?.top ?? 0;
              if (entry.boundingClientRect.top <= rootTop) {
                this.ceilingLocked.set(true);
              } else {
                this.ceilingLocked.set(false);
              }
            }
          });
        }, { root: container, threshold: [0, 0.1, 0.5, 0.9, 1] });
      }

      this.dynamicObserver.disconnect();
      if (topAnchor) this.dynamicObserver.observe(topAnchor);
      if (ceiling) this.dynamicObserver.observe(ceiling);
    });

    afterNextRender(() => {
      this.scrollToBottom();
      this.setupObservers();
    });
  }

  public scrollToBottom(): void {
    const container = this.containerRef().nativeElement;
    container.scrollTop = container.scrollHeight;
    this.isAtBottom.set(true);
    this.ceilingLocked.set(false);
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
        if (!this.isAtBottom()) {
          this.unreadIndicatorActive.set(true);
          return;
        }

        if (this.ceilingLocked()) {
          this.unreadIndicatorActive.set(true);
          return;
        }
        
        container.scrollTop = container.scrollHeight;
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
      if (this.dynamicObserver) this.dynamicObserver.disconnect();
      mutationObserver.disconnect();
    });
  }
}