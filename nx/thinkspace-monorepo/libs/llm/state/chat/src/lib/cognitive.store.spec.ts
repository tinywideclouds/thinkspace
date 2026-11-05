import { TestBed } from '@angular/core/testing';
import { CognitiveStore } from './cognitive.store';
import { describe, beforeEach, afterEach, it, expect, vi } from 'vitest';
import { AttentionFocus, DigestMeta, FileContextMetaData, PreviewContext } from '@org/llm-core-facade';

describe('CognitiveStore', () => {
  let store: CognitiveStore;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [CognitiveStore]
    });
    store = TestBed.inject(CognitiveStore);

    // Mock clipboard API
    Object.assign(navigator, {
      clipboard: {
        writeText: vi.fn().mockImplementation(() => Promise.resolve()),
      },
    });
  });

  afterEach(() => {
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
    // Arrange: Set dirty turn state
    store.pendingContextReview.set({ spaceId: 'golang' } as PreviewContext);
    store.pendingStrategy.set(true);
    store.pendingReviewBranch.set('candidate/123');
    store.pendingTransition.set({ targetThinkState: 'concept', reasoning: 'test' });

    // Act
    store.resetTurn();

    // Assert
    expect(store.pendingContextReview()).toBeNull();
    expect(store.pendingStrategy()).toBe(false);
    expect(store.pendingReviewBranch()).toBeNull();
    expect(store.pendingTransition()).toBeNull();
  });

  it('should clear all session state on clearSession()', () => {
    // Arrange: Set dirty session and turn state
    store.activeFocus.set({ primaryFocus: ['auth'] } as AttentionFocus);
    store.activeDigests.set({ 'digest-1': {} as DigestMeta });
    store.activeWorkbench.set([{ path: 'src/main.go' } as FileContextMetaData]);
    store.pendingStrategy.set(true);

    // Act
    store.clearSession();

    // Assert
    expect(store.activeFocus()).toBeNull();
    expect(store.activeDigests()).toEqual({});
    expect(store.activeWorkbench()).toEqual([]);
    expect(store.pendingStrategy()).toBe(false); // Validates resetTurn was called
  });

  it('should copy mock preview payload to clipboard', () => {
    const clipboardSpy = vi.spyOn(navigator.clipboard, 'writeText');
    
    store.copyFullContextPreview('preview-123');
    
    expect(clipboardSpy).toHaveBeenCalledWith('Mock Full Context Payload for preview-123');
  });
});