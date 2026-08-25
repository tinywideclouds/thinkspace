import { Component, inject, afterNextRender } from '@angular/core';
import { ChatStateService } from '@org/llm-state-chat';
import { 
  ChatFeedComponent, 
  ChatInputComponent, 
  ChatStrategyPromptComponent, 
  ChatReviewPromptComponent 
} from '@org/llm-ui-chat';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [
    ChatFeedComponent,
    ChatInputComponent,
    ChatStrategyPromptComponent,
    ChatReviewPromptComponent,
  ],
  templateUrl: './app.html',
  styleUrl: './app.css'
})
export class AppComponent {
  protected state = inject(ChatStateService);

  constructor() {
    // Safely initiate the WebSocket connection only in the browser context after the first render
    afterNextRender(() => {
      this.state.connect('ws://localhost:8080/ws');
    });
  }
}