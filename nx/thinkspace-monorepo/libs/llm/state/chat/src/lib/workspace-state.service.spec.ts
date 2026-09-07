import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { WorkspaceStateService } from './workspace-state.service';
import { describe, beforeEach, afterEach, it, expect } from 'vitest';

describe('WorkspaceStateService', () => {
  let service: WorkspaceStateService;
  let httpTestingController: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [
        WorkspaceStateService,
        provideHttpClient(),
        provideHttpClientTesting()
      ]
    });
    
    service = TestBed.inject(WorkspaceStateService);
    httpTestingController = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    httpTestingController.verify();
  });

  it('should load spaces and automatically select the first one on instantiation', async () => {
    // The constructor immediately calls loadSpaces(), so we expect the request right away.
    const spacesRequest = httpTestingController.expectOne('/api/spaces');
    expect(spacesRequest.request.method).toBe('GET');
    spacesRequest.flush([
      { id: 'golang', name: 'Go Developer', isConfigured: true },
      { id: 'python', name: 'Python Developer', isConfigured: false }
    ]);

    const chatsRequest = httpTestingController.expectOne('/api/spaces/golang/chats');
    expect(chatsRequest.request.method).toBe('GET');
    chatsRequest.flush([
      { id: 'chat-1', name: 'Test Chat', createdAt: '2026-09-07T00:00:00Z' }
    ]);

    // Give the microtask queue a moment to resolve the async/await promises
    await Promise.resolve();

    expect(service.spaces().length).toBe(2);
    expect(service.activeSpaceId()).toBe('golang');
    expect(service.chats().length).toBe(1);
    expect(service.activeChatId()).toBe('chat-1');
  });

  it('should load chats for a specific space manually', async () => {
    // Clear out the constructor's initial automatic requests first
    httpTestingController.expectOne('/api/spaces').flush([]);

    const loadPromise = service.loadChats('angular');
    
    const request = httpTestingController.expectOne('/api/spaces/angular/chats');
    expect(request.request.method).toBe('GET');
    request.flush([
      { id: 'chat-2', name: 'Angular Chat', createdAt: '2026-09-07T00:00:00Z' }
    ]);

    await loadPromise;

    expect(service.activeSpaceId()).toBe('angular');
    expect(service.chats()[0].id).toBe('chat-2');
    expect(service.activeChatId()).toBe('chat-2');
  });

  it('should clear active chat if space has no chats', async () => {
    httpTestingController.expectOne('/api/spaces').flush([]);

    const loadPromise = service.loadChats('empty-space');
    
    const request = httpTestingController.expectOne('/api/spaces/empty-space/chats');
    request.flush([]);

    await loadPromise;

    expect(service.chats().length).toBe(0);
    expect(service.activeChatId()).toBeNull();
  });

  it('should create a chat and set it as active', async () => {
    httpTestingController.expectOne('/api/spaces').flush([]);

    service.activeSpaceId.set('golang');
    service.chats.set([{ id: 'chat-1', name: 'Old Chat', createdAt: '2026-09-06T00:00:00Z' }]);

    const createPromise = service.createChat('New Discussion');
    
    const request = httpTestingController.expectOne('/api/spaces/golang/chats');
    expect(request.request.method).toBe('POST');
    expect(request.request.body).toEqual({ name: 'New Discussion' });
    
    request.flush({ id: 'chat-2', name: 'New Discussion', createdAt: '2026-09-07T00:00:00Z' });

    await createPromise;

    const currentChats = service.chats();
    expect(currentChats.length).toBe(2);
    expect(currentChats[0].id).toBe('chat-2');
    expect(service.activeChatId()).toBe('chat-2');
  });
});