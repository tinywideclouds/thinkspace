import { Component, inject, afterNextRender } from '@angular/core';
import { ChatConnectionService } from '@org/llm-state-chat';
import { ChatContainerComponent } from '@org/llm-feature-chat';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [ChatContainerComponent],
  templateUrl: './app.html',
  styleUrls: ['./app.css'],
})
export class AppComponent {
  protected connection = inject(ChatConnectionService);

  constructor() {
    afterNextRender(() => {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsUrl = `${protocol}//${window.location.host}/ws`;
      this.connection.connect(wsUrl);
    });
  }
}