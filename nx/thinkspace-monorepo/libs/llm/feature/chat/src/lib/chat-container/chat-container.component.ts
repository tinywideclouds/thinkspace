import { Component, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { 
  ChatConnectionService, 
  FlowStore, 
  CognitiveStore,
  ThinkspaceStateService 
} from '@org/llm-state-chat';
import { 
  ChatInputComponent, 
  ChatStrategyPromptComponent, 
  ChatReviewPromptComponent,
  ChatTransitionPromptComponent,
  FlowTrackerComponent,
  FlowInspectorComponent,
  ChatHeaderComponent,
  ContextInspectorComponent
} from '@org/llm-ui-core';
import { TimelineFacade, LinearFeedComponent } from '@org/llm-ui-timeline';
import { CognitiveGraphComponent } from '@org/llm-ui-cognitive-graph';
import { FocusHudComponent } from '@org/llm-ui-hud';
import { DeskComponent } from '@org/llm-ui-workbench';

@Component({
  selector: 'llm-chat-container',
  standalone: true,
  imports: [
    CommonModule,
    ChatInputComponent,
    ChatStrategyPromptComponent,
    ChatReviewPromptComponent,
    ChatTransitionPromptComponent,
    FlowTrackerComponent,
    FlowInspectorComponent,
    ChatHeaderComponent,
    ContextInspectorComponent,
    LinearFeedComponent,
    CognitiveGraphComponent,
    FocusHudComponent,
    DeskComponent
  ],
  templateUrl: './chat-container.component.html',
  styleUrl: './chat-container.component.scss',
  host: {
    class: 'block h-full w-full'
  }
})
export class ChatContainerComponent {
  public connection = inject(ChatConnectionService);
  public flow = inject(FlowStore);
  public cognitive = inject(CognitiveStore);
  public workspaceState = inject(ThinkspaceStateService);
  public timelineFacade = inject(TimelineFacade);

  onSpaceSelected(spaceId: string) {
    this.workspaceState.loadChats(spaceId);
    this.connection.clearSession();
  }

  onChatSelected(chatId: string) {
    this.workspaceState.activeChatId.set(chatId);
    this.connection.clearSession();
  }

  onNewChatRequested() {
    const name = prompt('Enter a name for the new chat (or leave blank for auto):');
    if (name !== null) { 
      this.workspaceState.createChat(name);
      this.connection.clearSession();
    }
  }
}