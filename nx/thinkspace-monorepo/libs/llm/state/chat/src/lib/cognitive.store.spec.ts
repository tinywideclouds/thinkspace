import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { signal } from '@angular/core';
import { CognitiveStore } from './cognitive.store';
import { ThinkspaceStateService } from './thinkspace-state.service';
import { describe, beforeEach, afterEach, it, expect, vi } from 'vitest';
import { AttentionFocus, DigestMeta, FileContextMetaData, PreviewContext } from '@org/llm-core-facade';

describe('CognitiveStore', () => {
  let store: CognitiveStore;
  let httpTestingController: HttpTestingController;
  let mockThinkspace: { activeSpaceId: ReturnType<typeof signal<string | null>>; activeChatId: ReturnType<typeof signal<string | null>> };

  beforeEach(() => {
    mockThinkspace = {
      activeSpaceId: signal<string | null>('golang'),
      activeChatId: signal<string | null>('chat-1')
    };

    TestBed.configureTestingModule({
      providers: [
        CognitiveStore,
        provideHttpClient(),
        provideHttpClientTesting(),
        { provide: ThinkspaceStateService, useValue: mockThinkspace }
      ]
    });
    store = TestBed.inject(CognitiveStore);
    httpTestingController = TestBed.inject(HttpTestingController);

    Object.assign(navigator, {
      clipboard: {
        writeText: vi.fn().mockImplementation(() => Promise.resolve()),
      },
    });
  });

  afterEach(() => {
    httpTestingController.verify();
    vi.restoreAllMocks();
  });

  it('should initialize with default empty state', () => {
    expect(store.activeFocus()).toBeNull();
    expect(store.activeDigests()).toEqual({});
    expect(store.activeWorkbench()).toEqual([]);
    expect(store.pendingContextReview()).toBeNull();
    expect(store.pendingStrategy()).toBe(false);
    expect(store.pendingReviewBranch()).toBeNull();
    expect(store.pendingTransition()).toBeNull();
    expect(store.autoApproveContext()).toBe(false);
  });

  it('should reset turn-specific state on resetTurn()', () => {
    store.pendingContextReview.set({ spaceId: 'golang' } as PreviewContext);
    store.pendingStrategy.set(true);
    store.pendingReviewBranch.set('candidate/123');
    store.pendingTransition.set({ targetThinkState: 'concept', reasoning: 'test' });

    store.resetTurn();

    expect(store.pendingContextReview()).toBeNull();
    expect(store.pendingStrategy()).toBe(false);
    expect(store.pendingReviewBranch()).toBeNull();
    expect(store.pendingTransition()).toBeNull();
  });

  it('should clear all session state on clearSession()', () => {
    store.activeFocus.set({ primaryFocus: ['auth'] } as AttentionFocus);
    store.activeDigests.set({ 'digest-1': {} as DigestMeta });
    store.activeWorkbench.set([{ path: 'src/main.go' } as FileContextMetaData]);
    store.pendingStrategy.set(true);

    store.clearSession();

    expect(store.activeFocus()).toBeNull();
    expect(store.activeDigests()).toEqual({});
    expect(store.activeWorkbench()).toEqual([]);
    expect(store.pendingStrategy()).toBe(false);
  });

  it('should fetch the persisted preview markdown from backend and copy it to clipboard', async () => {
    const clipboardSpy = vi.spyOn(navigator.clipboard, 'writeText');

    const copyPromise = store.copyFullContextPreview('preview-123');

    const req = httpTestingController.expectOne(
      r => r.url === '/api/previews/chat-1/preview-123' && r.params.get('space') === 'golang'
    );
    expect(req.request.method).toBe('GET');
    expect(req.request.responseType).toBe('text');
    req.flush('# Full Context Preview\n\nRendered markdown');

    await copyPromise;

    expect(clipboardSpy).toHaveBeenCalledWith('# Full Context Preview\n\nRendered markdown');
  });
});