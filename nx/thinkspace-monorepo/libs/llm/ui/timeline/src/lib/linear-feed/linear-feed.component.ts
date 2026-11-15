import { Component, inject, computed, signal, viewChild, viewChildren, ElementRef, effect } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ChatConnectionService } from '@org/llm-state-chat';
import { ScrollViewportComponent } from '../scroll-viewport/scroll-viewport.component';
import { TimelineFacade } from '../timeline.facade';
import { BlockRouterComponent } from '../blocks/block-router.component';
import { SkeletonBlockComponent } from '../blocks/skeleton-block.component';
import { TurnGroupComponent } from './turn-group.component';
import { ActionBarComponent } from '../action-bar.component';
import { 
  ChatStrategyPromptComponent, 
  ChatReviewPromptComponent, 
  ChatTransitionPromptComponent,
  FlowInspectorComponent
} from '@org/llm-ui-core';

@Component({
  selector: 'llm-linear-feed',
  standalone: true,
  imports: [
    CommonModule, 
    ScrollViewportComponent, 
    BlockRouterComponent,
    SkeletonBlockComponent,
    TurnGroupComponent,
    ActionBarComponent,
    ChatStrategyPromptComponent,
    ChatReviewPromptComponent,
    ChatTransitionPromptComponent,
    FlowInspectorComponent
  ],
  host: { class: 'block h-full w-full' },
  templateUrl: './linear-feed.component.html'
})
export class LinearFeedComponent {
  public facade = inject(TimelineFacade);
  public connection = inject(ChatConnectionService);

  public viewport = viewChild(ScrollViewportComponent);
  public messageBlocks = viewChildren(BlockRouterComponent, { read: ElementRef });
  public turnGroups = viewChildren(TurnGroupComponent, { read: ElementRef });

  public prefetchAnchorElement = computed(() => {
    const blocks = this.messageBlocks();
    const index = this.facade.paginationSettings().prefetchIndex;
    return blocks.length > index ? blocks[index].nativeElement : null;
  });

  public latestFlowFailed = computed<boolean>(() => {
    const turns = this.facade.visibleFeed();
    if (turns.length === 0) return false;
    const latestTurn = turns[turns.length - 1];
    const flowBlock = [...latestTurn.blocks].reverse().find((b) => b.type === 'flow_execution');
    return flowBlock?.type === 'flow_execution' && flowBlock.status === 'FAILED';
  });

  public hasNewMessages = signal<boolean>(false);
  private lastSeenMessageCount = 0;
  private lastSeenTurnId: string | null = null;
  private activeAnchorTurnId: string | null = null;

  constructor() {
    effect(() => {
      const turns = this.facade.visibleFeed();
      const groups = this.turnGroups();
      const vp = this.viewport();

      if (!vp || turns.length === 0 || groups.length !== turns.length) return;

      const latestTurn = turns[turns.length - 1];
      if (latestTurn.id === this.lastSeenTurnId) return;

      const isInitialHydration = this.lastSeenTurnId === null;
      this.lastSeenTurnId = latestTurn.id;

      if (isInitialHydration) {
        this.activeAnchorTurnId = null;
        requestAnimationFrame(() => vp.scrollToBottom('instant'));
      } else if (latestTurn.leader.type === 'prompt') {
        this.activeAnchorTurnId = latestTurn.id;
        const latestTurnEl = groups[groups.length - 1].nativeElement as HTMLElement;
        requestAnimationFrame(() => vp.scrollElementToTop(latestTurnEl, 'smooth'));
      } else if (vp.scrollState() === 'auto') {
        this.activeAnchorTurnId = null;
        requestAnimationFrame(() => vp.scrollToBottom('smooth'));
      }
    });

    effect(() => {
      const feedLength = this.facade.visibleFeed().reduce((acc, turn) => acc + 1 + turn.blocks.length, 0);
      const vp = this.viewport();
      
      if (!vp) return;

      if (vp.scrollState() === 'auto') {
        this.lastSeenMessageCount = feedLength;
        this.hasNewMessages.set(false);
      } else if (feedLength > this.lastSeenMessageCount) {
        this.hasNewMessages.set(true);
      }
    });
  }

  public onContentGrowth(): void {
    const vp = this.viewport();
    if (!vp) return;

    const turns = this.facade.visibleFeed();
    const groups = this.turnGroups();
    if (turns.length === 0 || groups.length !== turns.length) return;

    const latestTurn = turns[turns.length - 1];
    if (this.activeAnchorTurnId && latestTurn.id === this.activeAnchorTurnId) {
      const latestTurnEl = groups[groups.length - 1].nativeElement as HTMLElement;
      vp.scrollElementToTop(latestTurnEl, 'instant');
    } else if (vp.scrollState() === 'auto') {
      vp.scrollToBottom('instant');
    }
  }

  public onUserScrolled(): void {
    this.activeAnchorTurnId = null;
  }

  public handleBulkAction(action: 'tag' | 'tombstone' | 'fork'): void {
    console.log(`[LinearFeed] Bulk action requested: ${action} on ${this.facade.selectedTurnIds().size} turns.`);
    // TODO: Wire up to dialogs/commands once API facade endpoints for bulk operations exist
    this.facade.toggleEditMode(); // Exit mode on execution
  }
}