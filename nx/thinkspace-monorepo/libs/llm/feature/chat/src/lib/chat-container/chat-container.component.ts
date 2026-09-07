import { Component, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ChatStateService, WorkspaceStateService } from '@org/llm-state-chat';
import { 
  ChatFeedComponent, 
  ChatInputComponent, 
  ChatStrategyPromptComponent, 
  ChatReviewPromptComponent,
  FlowTrackerComponent,
  FlowInspectorComponent,
  ChatHeaderComponent
} from '@org/llm-ui-chat';

@Component({
  selector: 'llm-chat-container',
  standalone: true,
  imports: [
    CommonModule,
    ChatFeedComponent,
    ChatInputComponent,
    ChatStrategyPromptComponent,
    ChatReviewPromptComponent,
    FlowTrackerComponent,
    FlowInspectorComponent,
    ChatHeaderComponent
  ],
  templateUrl: './chat-container.component.html',
  styleUrl: './chat-container.component.css'
})
export class ChatContainerComponent {
  public chatState = inject(ChatStateService);
  public workspaceState = inject(WorkspaceStateService);

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
    if (name !== null) { // null means the user cancelled the prompt
      this.workspaceState.createChat(name);
      this.chatState.clearSession();
    }
  }
}