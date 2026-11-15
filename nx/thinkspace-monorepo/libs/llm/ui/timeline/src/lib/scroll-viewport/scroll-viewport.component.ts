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

export type ScrollState = 'auto' | 'manual';

const DEBUG_PREFETCH_CLASSES = [
  'ring-4',
  'ring-orange-500',
  'ring-dashed',
  'bg-orange-500/10',
  'relative'
];

@Component({
  selector: 'llm-scroll-viewport',
  standalone: true,
  imports: [CommonModule],
  host: { class: 'block h-full w-full' },
  templateUrl: './scroll-viewport.component.html'
})
export class ScrollViewportComponent {
  public containerRef = viewChild.required<ElementRef<HTMLDivElement>>('scrollContainer');
  public contentWrapperRef = viewChild.required<ElementRef<HTMLDivElement>>('contentWrapper');
  public bottomSentinelRef = viewChild.required<ElementRef<HTMLDivElement>>('bottomSentinel');

  public topContentAnchor = input<Element | null>(null);
  public prefetchAnchor = input<Element | null>(null);
  public isDebugMode = input<boolean>(false);
  
  public loadPreviousPage = output<void>();
  public scrolledToBottom = output<void>();
  public contentGrowth = output<void>();
  public userScrolled = output<void>();

  public topContentVisible = signal<boolean>(false);
  public scrollState = signal<ScrollState>('auto');

  private destroyRef = inject(DestroyRef);
  private debugObserver: IntersectionObserver | null = null;
  private prefetchObserver: IntersectionObserver | null = null;
  private resizeObserver: ResizeObserver | null = null;
  private observedPrefetchElement: Element | null = null;
  private previousDebugPrefetchElement: Element | null = null;

  constructor() {
    // Dynamic top content debug observer
    effect(() => {
      const topAnchor = this.topContentAnchor();
      const container = this.containerRef()?.nativeElement;

      if (!container) return;

      if (!this.debugObserver) {
        this.debugObserver = new IntersectionObserver((entries) => {
          entries.forEach(entry => {
            if (entry.target === this.topContentAnchor()) {
              this.topContentVisible.set(entry.isIntersecting);
            }
          });
        }, { root: container, threshold: [0, 0.1, 0.5, 0.9, 1] });
      }

      this.debugObserver.disconnect();
      if (topAnchor) this.debugObserver.observe(topAnchor);
    });

    // Dynamic false-color debug styling
    effect(() => {
      const anchor = this.prefetchAnchor();
      const debugActive = this.isDebugMode();

      if (this.previousDebugPrefetchElement && this.previousDebugPrefetchElement !== anchor) {
        this.previousDebugPrefetchElement.classList.remove(...DEBUG_PREFETCH_CLASSES);
        this.previousDebugPrefetchElement = null;
      }

      if (debugActive && anchor) {
        if (!anchor.classList.contains(DEBUG_PREFETCH_CLASSES[0])) {
          anchor.classList.add(...DEBUG_PREFETCH_CLASSES);
        }
        this.previousDebugPrefetchElement = anchor;
      } else if (!debugActive && this.previousDebugPrefetchElement) {
        this.previousDebugPrefetchElement.classList.remove(...DEBUG_PREFETCH_CLASSES);
        this.previousDebugPrefetchElement = null;
      }
    });

    // Dynamic prefetch IntersectionObserver (only re-binds when the actual DOM element changes)
    effect(() => {
      const anchor = this.prefetchAnchor();
      const container = this.containerRef()?.nativeElement;

      if (!container || anchor === this.observedPrefetchElement) return;

      if (!this.prefetchObserver) {
        this.prefetchObserver = new IntersectionObserver(([entry]) => {
          if (entry.isIntersecting) {
            this.loadPreviousPage.emit();
          }
        }, { root: container, threshold: 0.1 });
      }

      this.prefetchObserver.disconnect();
      this.observedPrefetchElement = anchor;
      if (anchor) {
        this.prefetchObserver.observe(anchor);
      }
    });

    afterNextRender(() => {
      this.scrollToBottom('instant');
      this.setupObservers();
    });
  }

  public scrollToBottom(behavior: ScrollBehavior = 'smooth'): void {
    const container = this.containerRef().nativeElement;
    container.scrollTo({ top: container.scrollHeight, behavior });
    this.scrollState.set('auto');
  }

  public scrollElementToTop(element: HTMLElement, behavior: ScrollBehavior = 'smooth'): void {
    const container = this.containerRef().nativeElement;
    container.scrollTo({ top: element.offsetTop, behavior });
  }

  private setupObservers(): void {
    const container = this.containerRef().nativeElement;
    const wrapper = this.contentWrapperRef().nativeElement;

    const bottomObserver = new IntersectionObserver(([entry]) => {
      if (entry.isIntersecting) {
        this.scrollState.set('auto');
        this.scrolledToBottom.emit();
      } else {
        this.scrollState.set('manual');
      }
    }, { root: container, threshold: 0.1 });
    bottomObserver.observe(this.bottomSentinelRef().nativeElement);

    if (typeof ResizeObserver !== 'undefined') {
      this.resizeObserver = new ResizeObserver(() => {
        this.contentGrowth.emit();
      });
      this.resizeObserver.observe(wrapper);
    }

    const onUserScrollIntent = () => {
      this.userScrolled.emit();
    };
    container.addEventListener('wheel', onUserScrollIntent, { passive: true });
    container.addEventListener('touchmove', onUserScrollIntent, { passive: true });
    container.addEventListener('pointerdown', onUserScrollIntent, { passive: true });

    this.destroyRef.onDestroy(() => {
      if (this.previousDebugPrefetchElement) {
        this.previousDebugPrefetchElement.classList.remove(...DEBUG_PREFETCH_CLASSES);
      }
      bottomObserver.disconnect();
      if (this.resizeObserver) this.resizeObserver.disconnect();
      if (this.debugObserver) this.debugObserver.disconnect();
      if (this.prefetchObserver) this.prefetchObserver.disconnect();
      container.removeEventListener('wheel', onUserScrollIntent);
      container.removeEventListener('touchmove', onUserScrollIntent);
      container.removeEventListener('pointerdown', onUserScrollIntent);
    });
  }
}