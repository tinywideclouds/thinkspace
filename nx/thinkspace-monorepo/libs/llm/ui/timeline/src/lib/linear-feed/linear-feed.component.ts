import { Component, inject, computed, signal, viewChild, viewChildren, ElementRef, effect } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ChatConnectionService } from '@org/llm-state-chat';
import { ScrollViewportComponent } from '../scroll-viewport/scroll-viewport.component';
import { TimelineFacade } from '../timeline.facade';
import { BlockRouterComponent } from '../blocks/block-router.component';
import { SkeletonBlockComponent } from '../blocks/skeleton-block.component';

@Component({
  selector: 'llm-linear-feed',
  standalone: true,
  imports: [
    CommonModule, 
    ScrollViewportComponent, 
    BlockRouterComponent,
    SkeletonBlockComponent
  ],
  templateUrl: './linear-feed.component.html'
})
export class LinearFeedComponent {
  public facade = inject(TimelineFacade);
  public connection = inject(ChatConnectionService);

  public viewport = viewChild(ScrollViewportComponent);
  public messageBlocks = viewChildren(BlockRouterComponent, { read: ElementRef });

  public prefetchAnchorElement = computed(() => {
    const blocks = this.messageBlocks();
    const index = this.facade.paginationSettings().prefetchIndex;
    return blocks.length > index ? blocks[index].nativeElement : null;
  });

  public hasNewMessages = signal<boolean>(false);
  private lastSeenMessageCount = 0;

  constructor() {
    effect(() => {
      const feed = this.facade.workingMemoryFeed();
      const vp = this.viewport();
      
      if (!vp) return;

      if (vp.scrollState() === 'auto') {
        this.lastSeenMessageCount = feed.length;
        this.hasNewMessages.set(false);
      } else if (feed.length > this.lastSeenMessageCount) {
        this.hasNewMessages.set(true);
      }
    });
  }
}