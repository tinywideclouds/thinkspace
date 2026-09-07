import { Injectable, signal, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';

export interface Space {
  id: string;
  name: string;
  isConfigured: boolean;
}

export interface ChatMeta {
  id: string;
  name: string;
  createdAt: string;
}

@Injectable({ providedIn: 'root' })
export class WorkspaceStateService {
  private httpClient = inject(HttpClient);

  public spaces = signal<Space[]>([]);
  public activeSpaceId = signal<string | null>(null);

  public chats = signal<ChatMeta[]>([]);
  public activeChatId = signal<string | null>(null);

  constructor() {
    this.loadSpaces();
  }

  public async loadSpaces(): Promise<void> {
    try {
      const response = await firstValueFrom(this.httpClient.get<Space[]>('/api/spaces'));
      this.spaces.set(response);
      
      const currentSpaces = this.spaces();
      if (currentSpaces.length > 0 && !this.activeSpaceId()) {
        const defaultSpaceId = currentSpaces[0].id;
        this.activeSpaceId.set(defaultSpaceId);
        await this.loadChats(defaultSpaceId);
      }
    } catch (error) {
      console.error('Failed to load spaces', error);
    }
  }

  public async loadChats(spaceId: string): Promise<void> {
    this.activeSpaceId.set(spaceId);
    try {
      const response = await firstValueFrom(
        this.httpClient.get<ChatMeta[]>(`/api/spaces/${encodeURIComponent(spaceId)}/chats`)
      );
      this.chats.set(response);
      
      const currentChats = this.chats();
      if (currentChats.length > 0) {
        this.activeChatId.set(currentChats[0].id);
      } else {
        this.activeChatId.set(null);
      }
    } catch (error) {
      console.error(`Failed to load chats for space ${spaceId}`, error);
    }
  }

  public async createChat(name: string): Promise<void> {
    const spaceId = this.activeSpaceId();
    if (!spaceId) {
      return;
    }

    try {
      const payload = name ? { name } : {};
      const response = await firstValueFrom(
        this.httpClient.post<ChatMeta>(`/api/spaces/${encodeURIComponent(spaceId)}/chats`, payload)
      );
      
      this.chats.update(currentChats => [response, ...currentChats]);
      this.activeChatId.set(response.id);
    } catch (error) {
      console.error('Failed to create chat', error);
    }
  }
}