import { Component, inject, computed, signal, viewChild, ElementRef } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ChatConnectionService } from '@org/llm-state-chat';
import { ChatFlowCardComponent } from '@org/llm-ui-core';
import { ScrollViewportComponent } from '../scroll-viewport/scroll-viewport.component';
import { TimelineFacade } from '../timeline.facade';
import { UserMessageBlockComponent } from '../blocks/user-message-block.component';
import { ModelMessageBlockComponent } from '../blocks/model-message-block.component';
import { FileDeltaBlockComponent } from '../blocks/file-delta-block.component';

@Component({
  selector: 'llm-linear-feed',
  standalone: true,
  imports: [
    CommonModule, 
    ChatFlowCardComponent, 
    ScrollViewportComponent, 
    UserMessageBlockComponent, 
    ModelMessageBlockComponent,
    FileDeltaBlockComponent
  ],
  templateUrl: './linear-feed.component.html'
})
export class LinearFeedComponent {
  public facade = inject(TimelineFacade);
  public connection = inject(ChatConnectionService);

  public contextBoxHeight = signal<string>('6em');

  public activeAnchorRef = viewChild<ElementRef<HTMLElement>>('activeAnchor');
  public activeAnchorElement = computed(() => this.activeAnchorRef()?.nativeElement || null);

  public topAnchorRef = viewChild<ElementRef<HTMLElement>>('topAnchor');
  public topAnchorElement = computed(() => this.topAnchorRef()?.nativeElement || null);

  public activeUserPromptId = computed(() => {
    const feed = this.facade.workingMemoryFeed();
    for (let i = feed.length - 1; i >= 0; i--) {
      if (feed[i].source === 'user') return feed[i].id;
    }
    return null;
  });
}