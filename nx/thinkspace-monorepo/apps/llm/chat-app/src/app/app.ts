import { Component, inject, afterNextRender } from '@angular/core';
import { ChatStateService } from '@org/llm-state-chat';
import { ChatContainerComponent } from '@org/llm-feature-chat';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [ChatContainerComponent],
  templateUrl: './app.html',
  styleUrls: ['./app.css'],
})
export class AppComponent {
  protected state = inject(ChatStateService);

  constructor() {
    afterNextRender(() => {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsUrl = `${protocol}//${window.location.host}/ws`;
      this.state.connect(wsUrl);
    });
  }
}