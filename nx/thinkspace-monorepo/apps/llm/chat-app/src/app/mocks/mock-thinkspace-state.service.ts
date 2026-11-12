import { Injectable, signal } from '@angular/core';

@Injectable({ providedIn: 'root' })
export class MockThinkspaceStateService {
  public activeSpaceId = signal<string | null>('project-alpha');
  public activeChatId = signal<string | null>('chat-1');

  public spaces = signal([
    { id: 'project-alpha', name: 'Alpha Project (Mock)', isConfigured: true },
    { id: 'project-beta', name: 'Beta Project (Mock)', isConfigured: false }
  ]);
  
  public chats = signal([
    { id: 'chat-1', name: 'Refactoring Timeline', createdAt: new Date().toISOString() },
    { id: 'chat-2', name: 'Database Migration', createdAt: new Date().toISOString() }
  ]);

  // Stub out any initialization methods that might trigger HTTP calls
  public loadSpaces(): void {
    console.log('[MockThinkspaceStateService] Intercepted loadSpaces()');
  }

  public loadChats(spaceId: string): void {
    console.log(`[MockThinkspaceStateService] Intercepted loadChats(${spaceId})`);
  }
}