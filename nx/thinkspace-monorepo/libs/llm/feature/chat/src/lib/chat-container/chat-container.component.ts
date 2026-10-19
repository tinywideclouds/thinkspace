import { Component, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ChatStateService, WorkspaceStateService } from '@org/llm-state-chat';
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
import { LinearFeedComponent } from '@org/llm-ui-linear-feed';
import { CognitiveGraphComponent } from '@org/llm-ui-cognitive-graph';
import { FocusHudComponent } from '@org/llm-ui-hud';
import { DeskComponent } from '@org/llm-ui-workbench';

export type ViewMode = 'linear' | 'cognitive';

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
  styleUrl: './chat-container.component.css'
})
export class ChatContainerComponent {
  public chatState = inject(ChatStateService);
  public workspaceState = inject(WorkspaceStateService);

  public viewMode = signal<ViewMode>('linear');
  public hoveredTag = signal<string | null>(null);

  onSpaceSelected(spaceId: string) {
    this.workspaceState.loadChats(spaceId);
    this.chatState.clearSession();
  }

  onChatSelected(chatId: string) {
    this.workspaceState.activeChatId.set(chatId);
    this.chatState.clearSession();
  }

  onNewChatRequested() {
    const name = prompt('Enter a name for the new chat (or leave blank for auto):');
    if (name !== null) { 
      this.workspaceState.createChat(name);
      this.chatState.clearSession();
    }
  }
}