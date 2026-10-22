import { Component, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ChatConnectionService } from '@org/llm-state-chat';
import { ChatFlowCardComponent } from '@org/llm-ui-core';
import { ScrollViewportComponent } from '../scroll-viewport/scroll-viewport.component';
import { TimelineFacade } from '../timeline.facade';
import { UserMessageBlockComponent } from '../blocks/user-message-block.component';
import { ModelMessageBlockComponent } from '../blocks/model-message-block.component';

@Component({
  selector: 'llm-linear-feed',
  standalone: true,
  imports: [
    CommonModule, 
    ChatFlowCardComponent, 
    ScrollViewportComponent, 
    UserMessageBlockComponent, 
    ModelMessageBlockComponent
  ],
  templateUrl: './linear-feed.component.html'
})
export class LinearFeedComponent {
  public facade = inject(TimelineFacade);
  public connection = inject(ChatConnectionService);
}